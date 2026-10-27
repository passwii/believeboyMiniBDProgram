package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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
	listCalls int
}

func (f *fakeAuditService) Record(_ context.Context, event audit.Event) error {
	f.events = append(f.events, event)
	return f.recordErr
}

func (f *fakeAuditService) List(context.Context, int32, int32) ([]audit.Record, error) {
	f.listCalls++
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

func TestAuditListRejectsOverflowAndExcessivePagination(t *testing.T) {
	adminStore := newMemoryAdminStore(t)
	service := &fakeAuditService{}
	router := NewRouter(Dependencies{
		AdminAuth:       adminauth.New(adminStore, 12*time.Hour),
		AdminCookieName: "bb_admin_session", AdminSessionTTL: 12 * time.Hour,
		AllowedAdminOrigin: httpTestOrigin, Audit: service,
	})
	cookie := loginRequest(t, router).Result().Cookies()[0]
	for _, query := range []string{
		"offset=4294967297",
		"offset=100001",
		"limit=2147483648",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-events?"+query, nil)
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("query %q status = %d, want %d", query, recorder.Code, http.StatusBadRequest)
		}
	}
	if service.listCalls != 0 {
		t.Fatalf("audit list called %d times for invalid pagination", service.listCalls)
	}
}

func TestAuditWriteFailureKeepsLoginResponseAndLogsRequestID(t *testing.T) {
	adminStore := newMemoryAdminStore(t)
	auditLog := &fakeAuditService{recordErr: errors.New("audit database unavailable")}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := NewRouter(Dependencies{
		AdminAuth:       adminauth.New(adminStore, 12*time.Hour),
		AdminCookieName: "bb_admin_session", AdminSessionTTL: 12 * time.Hour,
		AllowedAdminOrigin: httpTestOrigin, Audit: auditLog, Logger: logger,
	})
	requestID := "audit-write-failure-request"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(
		`{"email":"admin@example.com","credential":"correct horse battery staple"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", httpTestOrigin)
	request.Header.Set("X-Request-Id", requestID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if len(auditLog.events) != 1 || auditLog.events[0].EventType != "ADMIN_LOGIN_SUCCESS" {
		t.Fatalf("audit events = %+v", auditLog.events)
	}
	if !strings.Contains(logs.String(), `"request_id":"`+requestID+`"`) {
		t.Fatalf("audit failure log does not include request ID %q: %s", requestID, logs.String())
	}
}
