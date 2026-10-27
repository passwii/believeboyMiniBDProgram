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
	"github.com/believeboy/mini-bd-program/services/api/internal/audit"
)

type fakeAuditService struct {
	events    []audit.Event
	recordErr error
	items     []audit.Record
}

func (f *fakeAuditService) Record(_ context.Context, event audit.Event) error {
	f.events = append(f.events, event)
	return f.recordErr
}

func (f *fakeAuditService) List(context.Context, int32, int32) ([]audit.Record, error) {
	return f.items, nil
}

func TestAuditListRequiresAdminAndOmitsHashes(t *testing.T) {
	adminStore := newMemoryAdminStore(t)
	auth := adminauth.New(adminStore, 12*time.Hour)
	service := &fakeAuditService{items: []audit.Record{{
		ID: uuid.New(), ActorType: audit.ActorAdmin, EventType: "ADMIN_LOGIN_SUCCESS",
		RequestID: "req-1", Metadata: json.RawMessage(`{"result":"ok"}`),
		CreatedAt: time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC),
	}}}
	router := NewRouter(Dependencies{
		AdminAuth: auth, AdminCookieName: "bb_admin_session", AdminSessionTTL: 12 * time.Hour,
		AllowedAdminOrigin: httpTestOrigin, Audit: service,
	})

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-events", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.Code)
	}

	login := loginRequest(t, router)
	cookie := login.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-events?limit=10", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "ip_hash") || strings.Contains(recorder.Body.String(), "user_agent_hash") {
		t.Fatalf("response exposed network hashes: %s", recorder.Body.String())
	}
}
