package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
}

func (f *fakeViewerAccess) Status(context.Context, string) (share.ShareStatus, error) {
	return share.ShareStatus{
		State: "ACTIVE", Type: "PUBLIC", Label: "group",
		ExpiresAt: time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC),
	}, f.statusErr
}

func (f *fakeViewerAccess) Unlock(_ context.Context, _ string, _ string, _ share.AccessMeta, ticket bool) (share.UnlockResult, error) {
	f.unlockCalled = true
	f.unlockIssueTicket = ticket
	if f.unlockErr != nil {
		return share.UnlockResult{}, f.unlockErr
	}
	return share.UnlockResult{
		SessionToken: "viewer-session", SessionExpiresAt: f.expiresAt,
		Ticket: "viewer-ticket",
	}, nil
}

func (f *fakeViewerAccess) Authenticate(context.Context, string) (share.SessionRecord, error) {
	return f.session, nil
}

func (f *fakeViewerAccess) ContentURL(context.Context, string) (share.ContentURLResult, error) {
	return share.ContentURLResult{
		URL:           "https://content.example.test/c/signed",
		WatermarkText: "group", ExpiresAt: f.expiresAt,
	}, nil
}

func (f *fakeViewerAccess) EnterTicket(context.Context, string) (share.TicketLoginResult, error) {
	if f.ticketErr != nil {
		return share.TicketLoginResult{}, f.ticketErr
	}
	return share.TicketLoginResult{
		SessionToken: "viewer-session", ShareToken: "share/token",
		SessionExpiresAt: f.expiresAt,
	}, nil
}

func (f *fakeViewerAccess) ReadContent(context.Context, string) ([]byte, error) {
	return f.content, f.readErr
}

func newPublicTestRouter(fake *fakeViewerAccess) http.Handler {
	return NewRouter(Dependencies{
		Access: fake, ViewerCookieName: "bb_viewer_session",
		ViewOrigin: "https://view.example.test", IPHashKey: bytes.Repeat([]byte{0x19}, 32),
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
}
