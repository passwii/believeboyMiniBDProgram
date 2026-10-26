package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

const (
	httpTestAdminID    = "33333333-3333-4333-8333-333333333333"
	httpTestAdminEmail = "admin@example.com"
	httpTestCredential = "correct horse battery staple"
	httpTestOrigin     = "http://admin.localhost:3000"
)

type memoryAdminStore struct {
	admins   map[string]adminauth.AdminRecord
	sessions map[string]adminauth.SessionRecord
}

func newMemoryAdminStore(t *testing.T) *memoryAdminStore {
	t.Helper()
	hash, err := security.HashCredential(httpTestCredential)
	if err != nil {
		t.Fatalf("HashCredential() error = %v", err)
	}
	return &memoryAdminStore{
		admins: map[string]adminauth.AdminRecord{
			httpTestAdminEmail: {
				ID:             uuid.MustParse(httpTestAdminID),
				Email:          httpTestAdminEmail,
				CredentialHash: hash,
				Role:           "ADMIN",
				Status:         "ACTIVE",
			},
		},
		sessions: make(map[string]adminauth.SessionRecord),
	}
}

func (m *memoryAdminStore) FindAdminByEmail(_ context.Context, email string) (adminauth.AdminRecord, error) {
	admin, ok := m.admins[email]
	if !ok {
		return adminauth.AdminRecord{}, adminauth.ErrNotFound
	}
	return admin, nil
}

func (m *memoryAdminStore) CreateSession(_ context.Context, input adminauth.CreateSessionInput) error {
	admin := m.admins[httpTestAdminEmail]
	m.sessions[string(input.SessionHash)] = adminauth.SessionRecord{
		ID:          input.ID,
		AdminUserID: input.AdminUserID,
		ExpiresAt:   input.ExpiresAt,
		LastSeenAt:  input.LastSeenAt,
		Email:       admin.Email,
		Role:        admin.Role,
		Status:      admin.Status,
	}
	return nil
}

func (m *memoryAdminStore) FindSession(_ context.Context, hash []byte) (adminauth.SessionRecord, error) {
	session, ok := m.sessions[string(hash)]
	if !ok {
		return adminauth.SessionRecord{}, adminauth.ErrNotFound
	}
	return session, nil
}

func (m *memoryAdminStore) TouchSession(_ context.Context, id uuid.UUID, at time.Time) error {
	for hash, session := range m.sessions {
		if session.ID == id {
			session.LastSeenAt = at
			m.sessions[hash] = session
			return nil
		}
	}
	return adminauth.ErrNotFound
}

func (m *memoryAdminStore) RevokeSession(_ context.Context, id uuid.UUID, at time.Time) error {
	for hash, session := range m.sessions {
		if session.ID == id {
			session.RevokedAt = &at
			m.sessions[hash] = session
			return nil
		}
	}
	return adminauth.ErrNotFound
}

func newTestAdminRouter(t *testing.T) http.Handler {
	t.Helper()
	store := newMemoryAdminStore(t)
	service := adminauth.New(store, 12*time.Hour)
	return NewRouter(Dependencies{
		AdminAuth:          service,
		AdminCookieName:    "bb_admin_session",
		AdminSessionTTL:    12 * time.Hour,
		AllowedAdminOrigin: httpTestOrigin,
	})
}

func loginRequest(t *testing.T, router http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"email":"admin@example.com","credential":"correct horse battery staple"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", httpTestOrigin)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminLoginSetsSessionCookie(t *testing.T) {
	router := newTestAdminRouter(t)
	recorder := loginRequest(t, router)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "bb_admin_session" || cookie.Value == "" {
		t.Fatalf("cookie = %+v", cookie)
	}
	if !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags = %+v", cookie)
	}
	if cookie.Path != "/" || cookie.MaxAge != 43200 {
		t.Fatalf("cookie path/max-age = %q/%d", cookie.Path, cookie.MaxAge)
	}
}

func TestAdminLoginReturnsUniformError(t *testing.T) {
	router := newTestAdminRouter(t)
	body := `{"email":"admin@example.com","credential":"wrong"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", httpTestOrigin)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != "INVALID_CREDENTIALS" || response.RequestID == "" {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(strings.ToLower(response.Message), "user") {
		t.Fatalf("message leaks account status: %q", response.Message)
	}
}

func TestAdminMeRequiresCookie(t *testing.T) {
	router := newTestAdminRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminMeReturnsPrincipal(t *testing.T) {
	router := newTestAdminRouter(t)
	login := loginRequest(t, router)
	loginCookie := login.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	request.AddCookie(loginCookie)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var principal adminauth.Principal
	if err := json.Unmarshal(recorder.Body.Bytes(), &principal); err != nil {
		t.Fatalf("decode principal: %v", err)
	}
	if principal.Email != httpTestAdminEmail || principal.Role != "ADMIN" {
		t.Fatalf("principal = %+v", principal)
	}
}

func TestAdminLogoutClearsCookie(t *testing.T) {
	router := newTestAdminRouter(t)
	login := loginRequest(t, router)
	loginCookie := login.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logout", nil)
	request.Header.Set("Origin", httpTestOrigin)
	request.AddCookie(loginCookie)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Fatalf("cookies = %+v", cookies)
	}
}

func TestAdminWritesRejectForeignOrigin(t *testing.T) {
	router := newTestAdminRouter(t)
	body := `{"email":"admin@example.com","credential":"correct horse battery staple"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}
