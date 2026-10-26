package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/believeboy/mini-bd-program/services/api/internal/config"
)

type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/content/")
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f.objects[key] = data
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		data, ok := f.objects[key]
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
			return
		}
		_, _ = w.Write(data)
	case http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestS3StoreRoundTrip(t *testing.T) {
	fake := &fakeS3{objects: make(map[string][]byte)}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	store, err := NewS3Store(config.Config{
		S3Endpoint:     server.URL,
		S3Region:       "us-east-1",
		S3Bucket:       "content",
		S3AccessKey:    "access",
		S3SecretKey:    "secret",
		S3UsePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, "content/one.html", "text/html", []byte("hello")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	data, err := store.Get(ctx, "content/one.html")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("Get() = %q", data)
	}
	if err := store.Delete(ctx, "content/one.html"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, "content/one.html"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(after delete) error = %v", err)
	}
}
