package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/security"
	"github.com/believeboy/mini-bd-program/services/api/internal/share"
)

type fakeViewerAccess struct {
	statusErr         error
	unlockErr         error
	readErr           error
	ticketErr         error
	session           share.SessionRecord
	content           []byte
	expiresAt         time.Time
	unlockCalled      bool
	unlockIssueTicket bool
	unlockMetas       []share.AccessMeta
	audit             *fakeAuditService
}

func (f *fakeViewerAccess) Status(context.Context, string) (share.ShareStatus, error) {
	return share.ShareStatus{
		State: "ACTIVE", Type: "PUBLIC", Label: "group",
		ExpiresAt: time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC),
	}, f.statusErr
}

func (f *fakeViewerAccess) Unlock(_ context.Context, _ string, _ string, meta share.AccessMeta, ticket bool) (share.UnlockResult, error) {
	f.unlockCalled = true
	f.unlockIssueTicket = ticket
	f.unlockMetas = append(f.unlockMetas, meta)
	if f.unlockErr != nil {
		return share.UnlockResult{}, f.unlockErr
	}
	return share.UnlockResult{
		SessionToken: "viewer-session", SessionExpiresAt: f.expiresAt,
		Ticket: "viewer-ticket", SessionID: uuid.New(), Share: share.ShareRecord{ID: uuid.New()},
	}, nil
}

func (f *fakeViewerAccess) Authenticate(context.Context, string) (share.SessionRecord, error) {
	return f.session, nil
}

func (f *fakeViewerAccess) ContentURL(context.Context, string) (share.ContentURLResult, error) {
	return share.ContentURLResult{
		URL:           "https://content.example.test/c/signed",
		WatermarkText: "group", ExpiresAt: f.expiresAt, SessionID: uuid.New(), ShareID: uuid.New(),
	}, nil
}

func (f *fakeViewerAccess) EnterTicket(context.Context, string) (share.TicketLoginResult, error) {
	if f.ticketErr != nil {
		return share.TicketLoginResult{}, f.ticketErr
	}
	return share.TicketLoginResult{
		SessionToken: "viewer-session", ShareToken: "share/token",
		SessionID: uuid.New(), ShareID: uuid.New(), SessionExpiresAt: f.expiresAt,
	}, nil
}

func (f *fakeViewerAccess) ReadContent(context.Context, string) (share.ContentReadResult, error) {
	return share.ContentReadResult{Data: f.content, ShareID: uuid.New(), SessionID: uuid.New()}, f.readErr
}

func newPublicTestRouter(fake *fakeViewerAccess) http.Handler {
	auditLog := &fakeAuditService{}
	fake.audit = auditLog
	return NewRouter(Dependencies{
		Access: fake, ViewerCookieName: "bb_viewer_session",
		ViewOrigin: "https://view.example.test", IPHashKey: bytes.Repeat([]byte{0x19}, 32),
		Audit: auditLog,
	})
}

func TestPublicShareStatus(t *testing.T) {
	router := newPublicTestRouter(&fakeViewerAccess{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/shares/token/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"state":"ACTIVE"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestPublicH5UnlockSetsCookie(t *testing.T) {
	fake := &fakeViewerAccess{expiresAt: time.Now().Add(time.Hour)}
	router := newPublicTestRouter(fake)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares/token/unlock",
		strings.NewReader(`{"pin":"1234"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || !fake.unlockCalled || fake.unlockIssueTicket {
		t.Fatalf("status = %d, fake = %+v", recorder.Code, fake)
	}
	if len(recorder.Result().Cookies()) != 1 {
		t.Fatalf("cookies = %+v", recorder.Result().Cookies())
	}
	if len(fake.audit.events) != 1 || fake.audit.events[0].EventType != "VIEWER_PIN_SUCCESS" {
		t.Fatalf("audit events = %+v", fake.audit.events)
	}
}

func TestPublicUnlockUsesForwardedClientIPFromTrustedProxy(t *testing.T) {
	key := bytes.Repeat([]byte{0x29}, 32)
	fake := &fakeViewerAccess{expiresAt: time.Now().Add(time.Hour)}
	auditLog := &fakeAuditService{}
	router := NewRouter(Dependencies{
		Access: fake, ViewerCookieName: "bb_viewer_session",
		ViewOrigin: "https://view.example.test", IPHashKey: key,
		TrustedProxyCIDRs: []string{"10.0.0.0/8"}, Audit: auditLog,
	})

	for _, proxyIP := range []string{"10.0.0.1", "10.0.0.2"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/shares/token/unlock",
			strings.NewReader(`{"pin":"1234"}`))
		request.RemoteAddr = proxyIP + ":4321"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", "198.51.100.42")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("unlock status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	}
	if len(fake.unlockMetas) != 2 {
		t.Fatalf("unlock metadata count = %d, want 2", len(fake.unlockMetas))
	}
	want := security.HMAC(key, "198.51.100.42")
	if !bytes.Equal(fake.unlockMetas[0].IPHash, want) || !bytes.Equal(fake.unlockMetas[1].IPHash, want) {
		t.Fatalf("client IP hashes = %x, %x; want %x", fake.unlockMetas[0].IPHash, fake.unlockMetas[1].IPHash, want)
	}
}

func TestPublicMiniUnlockReturnsTicket(t *testing.T) {
	fake := &fakeViewerAccess{expiresAt: time.Now().Add(time.Hour)}
	router := newPublicTestRouter(fake)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/miniapp/shares/token/unlock",
		strings.NewReader(`{"pin":"1234"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !fake.unlockIssueTicket {
		t.Fatalf("status = %d, fake = %+v", recorder.Code, fake)
	}
	if !strings.Contains(recorder.Body.String(), `"ticket":"viewer-ticket"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	if len(fake.audit.events) != 1 || fake.audit.events[0].EventType != "VIEWER_PIN_SUCCESS" {
		t.Fatalf("audit events = %+v", fake.audit.events)
	}
}

func TestPublicContentHeadersAndEnterRedirect(t *testing.T) {
	fake := &fakeViewerAccess{
		expiresAt: time.Now().Add(time.Hour),
		content:   []byte("<html>safe</html>"),
	}
	router := newPublicTestRouter(fake)

	contentRequest := httptest.NewRequest(http.MethodGet, "/c/signed", nil)
	contentRecorder := httptest.NewRecorder()
	router.ServeHTTP(contentRecorder, contentRequest)
	if contentRecorder.Code != http.StatusOK {
		t.Fatalf("content status = %d", contentRecorder.Code)
	}
	if contentRecorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache-control = %q", contentRecorder.Header().Get("Cache-Control"))
	}
	if !strings.Contains(contentRecorder.Header().Get("Content-Security-Policy"), "connect-src 'none'") {
		t.Fatalf("csp = %q", contentRecorder.Header().Get("Content-Security-Policy"))
	}

	enterRequest := httptest.NewRequest(http.MethodGet, "/enter?ticket=t", nil)
	enterRecorder := httptest.NewRecorder()
	router.ServeHTTP(enterRecorder, enterRequest)
	if enterRecorder.Code != http.StatusSeeOther {
		t.Fatalf("enter status = %d", enterRecorder.Code)
	}
	if enterRecorder.Header().Get("Location") != "https://view.example.test/s/share%2Ftoken" {
		t.Fatalf("location = %q", enterRecorder.Header().Get("Location"))
	}
	if len(fake.audit.events) != 2 || fake.audit.events[0].EventType != "VIEWER_CONTENT_SERVED" ||
		fake.audit.events[1].EventType != "VIEWER_TICKET_CONSUMED" {
		t.Fatalf("audit events = %+v", fake.audit.events)
	}
}

func TestPublicRateLimitResponse(t *testing.T) {
	fake := &fakeViewerAccess{
		expiresAt: time.Now().Add(time.Hour),
		unlockErr: &share.RateLimitError{RetryAfter: 15 * time.Minute},
	}
	router := newPublicTestRouter(fake)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares/token/unlock",
		strings.NewReader(`{"pin":"1234"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "900" {
		t.Fatalf("retry-after = %q", recorder.Header().Get("Retry-After"))
	}
	if len(fake.audit.events) != 1 || fake.audit.events[0].EventType != "VIEWER_PIN_RATE_LIMITED" {
		t.Fatalf("audit events = %+v", fake.audit.events)
	}
}

func TestPublicPINFailureIsAuditedWithoutPIN(t *testing.T) {
	fake := &fakeViewerAccess{
		expiresAt: time.Now().Add(time.Hour),
		unlockErr: &share.ShareOperationError{ShareID: uuid.New(), Cause: share.ErrPINRejected},
	}
	router := newPublicTestRouter(fake)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares/token/unlock",
		strings.NewReader(`{"pin":"1234"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if len(fake.audit.events) != 1 || fake.audit.events[0].EventType != "VIEWER_PIN_FAILURE" {
		t.Fatalf("audit events = %+v", fake.audit.events)
	}
	if strings.Contains(string(fake.audit.events[0].Metadata), "1234") {
		t.Fatalf("audit contains PIN: %s", fake.audit.events[0].Metadata)
	}
}
