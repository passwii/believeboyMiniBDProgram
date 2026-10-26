package security

import (
	"bytes"
	"testing"
)

func TestHMACIsStableAndKeyed(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	first := HMAC(key, "127.0.0.1")
	second := HMAC(key, "127.0.0.1")
	other := HMAC(key, "127.0.0.2")
	if len(first) != 32 || !bytes.Equal(first, second) {
		t.Fatal("HMAC is not stable")
	}
	if bytes.Equal(first, other) {
		t.Fatal("HMAC should differ for different values")
	}
}
