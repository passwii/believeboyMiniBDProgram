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
	"github.com/believeboy/mini-bd-program/services/api/internal/share"
)

type fakeAdminShareService struct {
	created   share.CreateInput
	items     []share.Share
	revoked   uuid.UUID
	reset     uuid.UUID
	resetPIN  string
	createErr error
	audit     *fakeAuditService
}

func (f *fakeAdminShareService) Create(_ context.Context, input share.CreateInput) (share.Share, error) {
	f.created = input
	if f.createErr != nil {
		return share.Share{}, f.createErr
	}
	return share.Share{
		ID: uuid.New(), ContentID: input.ContentID, ContentVersionID: uuid.New(),
		ContentTitle: "Proposal", Type: input.Type, Label: input.Label,
		Token: "share-token", WatermarkText: input.WatermarkText,
		ExpiresAt: input.ExpiresAt, SessionTTL: input.SessionTTL,
		CreatedAt: time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeAdminShareService) List(context.Context, int32, int32) ([]share.Share, error) {
	return f.items, nil
}

func (f *fakeAdminShareService) Revoke(_ context.Context, id uuid.UUID) error {
	f.revoked = id
	return nil
}

func (f *fakeAdminShareService) ResetPIN(_ context.Context, id uuid.UUID, pin string) error {
	f.reset = id
	f.resetPIN = pin
	return nil
}

func newShareTestRouter(t *testing.T) (http.Handler, *fakeAdminShareService) {
	t.Helper()
	store := newMemoryAdminStore(t)
	auth := adminauth.New(store, 12*time.Hour)
	service := &fakeAdminShareService{}
	auditLog := &fakeAuditService{}
	service.audit = auditLog
	return NewRouter(Dependencies{
		AdminAuth: auth, AdminCookieName: "bb_admin_session",
		AdminSessionTTL: 12 * time.Hour, AllowedAdminOrigin: httpTestOrigin,
		Shares: service, ViewOrigin: "https://view.example.test", Audit: auditLog,
	}), service
}

func TestAdminShareRoutesRequireSession(t *testing.T) {
	router, _ := newShareTestRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/shares", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminCreateShare(t *testing.T) {
	router, service := newShareTestRouter(t)
	cookie := authCookie(t, router)
	contentID := uuid.New()
	body := `{
		"content_id":"` + contentID.String() + `",
		"type":"PUBLIC",
		"label":"群分享",
		"pin":"1234",
		"expires_at":"2026-11-27T01:00:00Z",
		"session_ttl_seconds":1800
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shares", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", httpTestOrigin)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.created.PIN != "1234" || service.created.ContentID != contentID {
		t.Fatalf("created = %+v", service.created)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["h5_url"] != "https://view.example.test/s/share-token" {
		t.Fatalf("h5_url = %v", payload["h5_url"])
	}
	if payload["miniapp_path"] != "/pages/open/index?t=share-token" {
		t.Fatalf("miniapp_path = %v", payload["miniapp_path"])
	}
	if len(service.audit.events) < 2 || service.audit.events[len(service.audit.events)-1].EventType != "SHARE_CREATED" {
		t.Fatalf("audit events = %+v", service.audit.events)
	}
}

func TestAdminCreateShareRejectsInvalidPIN(t *testing.T) {
	router, service := newShareTestRouter(t)
	service.createErr = share.ErrInvalidPIN
	cookie := authCookie(t, router)
	body := `{
		"content_id":"` + uuid.NewString() + `",
		"type":"PUBLIC",
		"label":"test",
		"pin":"12",
		"expires_at":"2026-11-27T01:00:00Z"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shares", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", httpTestOrigin)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminRevokeAndResetSharePIN(t *testing.T) {
	router, service := newShareTestRouter(t)
	cookie := authCookie(t, router)
	id := uuid.New()

	revoke := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shares/"+id.String()+"/revoke", nil)
	revoke.Header.Set("Origin", httpTestOrigin)
	revoke.AddCookie(cookie)
	revokeResponse := httptest.NewRecorder()
	router.ServeHTTP(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusNoContent || service.revoked != id {
		t.Fatalf("revoke status = %d, revoked = %v", revokeResponse.Code, service.revoked)
	}

	reset := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shares/"+id.String()+"/reset-pin",
		strings.NewReader(`{"pin":"5678"}`))
	reset.Header.Set("Content-Type", "application/json")
	reset.Header.Set("Origin", httpTestOrigin)
	reset.AddCookie(cookie)
	resetResponse := httptest.NewRecorder()
	router.ServeHTTP(resetResponse, reset)
	if resetResponse.Code != http.StatusNoContent || service.reset != id || service.resetPIN != "5678" {
		t.Fatalf("reset status = %d, reset = %v, pin = %q", resetResponse.Code, service.reset, service.resetPIN)
	}
	if len(service.audit.events) != 3 || service.audit.events[1].EventType != "SHARE_REVOKED" ||
		service.audit.events[2].EventType != "SHARE_PIN_RESET" {
		t.Fatalf("audit events = %+v", service.audit.events)
	}
}
