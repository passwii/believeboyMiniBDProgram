package share

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestContentClaimsRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x33}, 32)
	claims := ContentClaims{
		ShareID:   uuid.New(),
		VersionID: uuid.New(),
		SessionID: uuid.New(),
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	}
	token, err := SignClaims(claims, key)
	if err != nil {
		t.Fatalf("SignClaims() error = %v", err)
	}
	verified, err := VerifyClaims(token, key)
	if err != nil {
		t.Fatalf("VerifyClaims() error = %v", err)
	}
	if verified != claims {
		t.Fatalf("claims = %+v, want %+v", verified, claims)
	}
}

func TestVerifyClaimsRejectsExpiredAndTampered(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	expired, err := SignClaims(ContentClaims{
		ShareID:   uuid.New(),
		VersionID: uuid.New(),
		SessionID: uuid.New(),
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}, key)
	if err != nil {
		t.Fatalf("SignClaims() error = %v", err)
	}
	if _, err := VerifyClaims(expired, key); !errors.Is(err, ErrClaimExpired) {
		t.Fatalf("expired error = %v", err)
	}
	if _, err := VerifyClaims(expired+"x", key); !errors.Is(err, ErrClaimInvalid) {
		t.Fatalf("tampered error = %v", err)
	}
}
