package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	AdminOrigin     string
	AdminCookieName string
	AdminSessionTTL time.Duration
	SecureCookies   bool
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

	cfg := Config{
		HTTPAddr:        stringEnv("API_ADDR", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminOrigin:     strings.TrimRight(stringEnv("ADMIN_ORIGIN", "http://admin.localhost:3000"), "/"),
		AdminCookieName: stringEnv("ADMIN_COOKIE_NAME", "bb_admin_session"),
		AdminSessionTTL: ttl,
		SecureCookies:   secure,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
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
