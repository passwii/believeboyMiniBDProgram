package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ADMIN_SESSION_TTL", "")
	t.Setenv("ADMIN_COOKIE_NAME", "")
	t.Setenv("SECURE_COOKIES", "")

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
}
