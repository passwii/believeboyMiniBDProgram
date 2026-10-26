package database

import (
	"context"
	"os"
	"testing"
)

func TestMigrateAndPing(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if err := Migrate(url, "up"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	pool, err := Open(context.Background(), url)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
