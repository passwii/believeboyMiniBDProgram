package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	AdminOrigin        string
	AdminCookieName    string
	AdminSessionTTL    time.Duration
	SecureCookies      bool
	ViewOrigin         string
	ContentOrigin      string
	ViewerCookieName   string
	ViewerSessionTTL   time.Duration
	ViewerTicketTTL    time.Duration
	ContentURLTTL      time.Duration
	TokenEncryptionKey []byte
	ContentSigningKey  []byte
	IPHashKey          []byte
	MaxUploadBytes     int64
	S3Endpoint         string
	S3Region           string
	S3Bucket           string
	S3AccessKey        string
	S3SecretKey        string
	S3UsePathStyle     bool
}

func Load() (Config, error) {
	ttl, err := durationEnv("ADMIN_SESSION_TTL", 12*time.Hour)
	if err != nil {
		return Config{}, err
	}
	secure, err := boolEnv("SECURE_COOKIES", false)
	if err != nil {
		return Config{}, err
	}
	maxUploadBytes, err := int64Env("MAX_UPLOAD_BYTES", 20<<20)
	if err != nil {
		return Config{}, err
	}
	if maxUploadBytes <= 0 || maxUploadBytes > 20<<20 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be between 1 and 20971520")
	}
	usePathStyle, err := boolEnv("S3_USE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}
	viewerSessionTTL, err := durationEnv("VIEWER_SESSION_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	viewerTicketTTL, err := durationEnv("VIEWER_TICKET_TTL", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	contentURLTTL, err := durationEnv("CONTENT_URL_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	tokenKey, err := keyEnv("TOKEN_ENCRYPTION_KEY")
	if err != nil {
		return Config{}, err
	}
	signingKey, err := keyEnv("CONTENT_SIGNING_KEY")
	if err != nil {
		return Config{}, err
	}
	ipHashKey, err := keyEnv("IP_HASH_KEY")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:           stringEnv("API_ADDR", ":8080"),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminOrigin:        strings.TrimRight(stringEnv("ADMIN_ORIGIN", "http://admin.localhost:3000"), "/"),
		AdminCookieName:    stringEnv("ADMIN_COOKIE_NAME", "bb_admin_session"),
		AdminSessionTTL:    ttl,
		SecureCookies:      secure,
		ViewOrigin:         strings.TrimRight(stringEnv("VIEW_ORIGIN", "http://view.localhost:5173"), "/"),
		ContentOrigin:      strings.TrimRight(stringEnv("CONTENT_ORIGIN", "http://content.localhost:8080"), "/"),
		ViewerCookieName:   stringEnv("VIEWER_COOKIE_NAME", "bb_viewer_session"),
		ViewerSessionTTL:   viewerSessionTTL,
		ViewerTicketTTL:    viewerTicketTTL,
		ContentURLTTL:      contentURLTTL,
		TokenEncryptionKey: tokenKey,
		ContentSigningKey:  signingKey,
		IPHashKey:          ipHashKey,
		MaxUploadBytes:     maxUploadBytes,
		S3Endpoint:         stringEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:           stringEnv("S3_REGION", "us-east-1"),
		S3Bucket:           strings.TrimSpace(os.Getenv("S3_BUCKET")),
		S3AccessKey:        strings.TrimSpace(os.Getenv("S3_ACCESS_KEY")),
		S3SecretKey:        strings.TrimSpace(os.Getenv("S3_SECRET_KEY")),
		S3UsePathStyle:     usePathStyle,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return Config{}, fmt.Errorf("S3_BUCKET, S3_ACCESS_KEY, and S3_SECRET_KEY are required")
	}
	return cfg, nil
}

func stringEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func int64Env(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func keyEnv(key string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", key)
	}
	value, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64: %w", key, err)
	}
	if len(value) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes", key)
	}
	return value, nil
}
