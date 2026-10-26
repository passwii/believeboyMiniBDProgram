package share

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrClaimInvalid = errors.New("content claim invalid")
	ErrClaimExpired = errors.New("content claim expired")
)

type ContentClaims struct {
	ShareID   uuid.UUID `json:"share_id"`
	VersionID uuid.UUID `json:"version_id"`
	SessionID uuid.UUID `json:"session_id"`
	ExpiresAt int64     `json:"exp"`
}

func SignClaims(claims ContentClaims, key []byte) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode claims: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := sign([]byte(encoded), key)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func VerifyClaims(token string, key []byte) (ContentClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ContentClaims{}, ErrClaimInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, sign([]byte(parts[0]), key)) {
		return ContentClaims{}, ErrClaimInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ContentClaims{}, ErrClaimInvalid
	}
	var claims ContentClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ContentClaims{}, ErrClaimInvalid
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return ContentClaims{}, ErrClaimExpired
	}
	return claims, nil
}

func sign(payload, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
