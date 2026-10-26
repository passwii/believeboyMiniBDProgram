package security

import (
	"bytes"
	"testing"
)

func TestNewTokenReturnsStableHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if raw == "" || len(hash) != 32 {
		t.Fatalf("raw length = %d, hash length = %d", len(raw), len(hash))
	}
	if string(hash) == raw {
		t.Fatal("hash must not equal raw token")
	}
	if !bytes.Equal(HashToken(raw), hash) {
		t.Fatal("HashToken(raw) does not match NewToken hash")
	}
}
