package config

import "testing"

const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("S3_BUCKET", "content")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("TOKEN_ENCRYPTION_KEY", testKey)
	t.Setenv("CONTENT_SIGNING_KEY", testKey)
	t.Setenv("IP_HASH_KEY", testKey)
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("S3_BUCKET", "content")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ADMIN_SESSION_TTL", "")
	t.Setenv("ADMIN_COOKIE_NAME", "")
	t.Setenv("SECURE_COOKIES", "")
	t.Setenv("MAX_UPLOAD_BYTES", "")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_REGION", "")
	t.Setenv("S3_USE_PATH_STYLE", "")
	t.Setenv("VIEWER_COOKIE_NAME", "")
	t.Setenv("VIEWER_SESSION_TTL", "")
	t.Setenv("VIEWER_TICKET_TTL", "")
	t.Setenv("CONTENT_URL_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AdminSessionTTL.Hours() != 12 {
		t.Fatalf("AdminSessionTTL = %v, want 12h", cfg.AdminSessionTTL)
	}
	if cfg.AdminCookieName != "bb_admin_session" {
		t.Fatalf("AdminCookieName = %q", cfg.AdminCookieName)
	}
	if cfg.SecureCookies {
		t.Fatal("SecureCookies should default to false")
	}
	if cfg.MaxUploadBytes != 20<<20 {
		t.Fatalf("MaxUploadBytes = %d", cfg.MaxUploadBytes)
	}
	if cfg.S3Endpoint != "http://localhost:9000" {
		t.Fatalf("S3Endpoint = %q", cfg.S3Endpoint)
	}
	if cfg.S3Region != "us-east-1" {
		t.Fatalf("S3Region = %q", cfg.S3Region)
	}
	if !cfg.S3UsePathStyle {
		t.Fatal("S3UsePathStyle should default to true")
	}
	if cfg.ViewerCookieName != "bb_viewer_session" {
		t.Fatalf("ViewerCookieName = %q", cfg.ViewerCookieName)
	}
	if cfg.ViewerSessionTTL.Minutes() != 30 {
		t.Fatalf("ViewerSessionTTL = %v", cfg.ViewerSessionTTL)
	}
	if cfg.ViewerTicketTTL.Seconds() != 60 {
		t.Fatalf("ViewerTicketTTL = %v", cfg.ViewerTicketTTL)
	}
	if cfg.ContentURLTTL.Minutes() != 10 {
		t.Fatalf("ContentURLTTL = %v", cfg.ContentURLTTL)
	}
}

func TestLoadRejectsOversizedUploadLimit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAX_UPLOAD_BYTES", "20971521")

	_, err := Load()
	if err == nil {
		t.Fatal("expected oversized MAX_UPLOAD_BYTES to fail")
	}
}

func TestLoadRequiresS3Credentials(t *testing.T) {
	for _, key := range []string{"S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Run(key, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected missing %s to fail", key)
			}
		})
	}
}

func TestLoadRejectsInvalidEncryptionKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("TOKEN_ENCRYPTION_KEY", "c2hvcnQ=")

	_, err := Load()
	if err == nil {
		t.Fatal("expected short encryption key to fail")
	}
}
