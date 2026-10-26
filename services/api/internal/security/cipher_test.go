package security

import (
	"bytes"
	"testing"
)

func TestCipherRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	cipher, err := NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	first, err := cipher.Encrypt([]byte("share-token"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := cipher.Encrypt([]byte("share-token"))
	if err != nil {
		t.Fatalf("Encrypt(second) error = %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("ciphertexts should differ because of random nonces")
	}
	plaintext, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(plaintext) != "share-token" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestCipherRejectsTamperingAndInvalidKey(t *testing.T) {
	if _, err := NewCipher([]byte("short")); err == nil {
		t.Fatal("expected short key to fail")
	}
	cipher, err := NewCipher(bytes.Repeat([]byte{0x24}, 32))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	encrypted, err := cipher.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	encrypted[len(encrypted)-1] ^= 0xff
	if _, err := cipher.Decrypt(encrypted); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
}
