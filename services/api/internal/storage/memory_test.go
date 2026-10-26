package storage

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	input := []byte("<html>one</html>")

	if err := store.Put(ctx, "content/one.html", "text/html", input); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	input[0] = 'X'

	output, err := store.Get(ctx, "content/one.html")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !bytes.Equal(output, []byte("<html>one</html>")) {
		t.Fatalf("Get() = %q", output)
	}
	output[0] = 'Y'
	again, err := store.Get(ctx, "content/one.html")
	if err != nil {
		t.Fatalf("Get(again) error = %v", err)
	}
	if again[0] != '<' {
		t.Fatal("Get() returned a mutable internal slice")
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	if err := store.Put(ctx, "key", "text/html", []byte("value")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, "key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete(missing) error = %v", err)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := store.Put(ctx, "shared", "text/html", []byte("value")); err != nil {
				t.Errorf("Put() error = %v", err)
			}
			if _, err := store.Get(ctx, "shared"); err != nil {
				t.Errorf("Get() error = %v", err)
			}
		}()
	}
	wait.Wait()
}
