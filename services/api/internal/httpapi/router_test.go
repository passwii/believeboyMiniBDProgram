package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerStub struct {
	err error
}

func (p pingerStub) Ping(context.Context) error {
	return p.err
}

func TestLiveness(t *testing.T) {
	router := NewRouter(Dependencies{Database: pingerStub{}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestReadinessFailsWhenDatabasePingFails(t *testing.T) {
	router := NewRouter(Dependencies{Database: pingerStub{err: errors.New("offline")}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
