package security

import (
	"crypto/hmac"
	"crypto/sha256"
)

func HMAC(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
