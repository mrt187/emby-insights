package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
}

func TestLoadRequiresAppEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("APP_ENCRYPTION_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing APP_ENCRYPTION_KEY error")
	}
}

func TestLoadUsesConfiguredValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("LISTEN_ADDRESS", "127.0.0.1:9090")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("APP_ENCRYPTION_KEY", "test-encryption-key")
	t.Setenv("VAPID_PUBLIC_KEY", "test-public-key")
	t.Setenv("VAPID_PRIVATE_KEY", "test-private-key")
	t.Setenv("VAPID_SUBJECT", "mailto:test@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:9090" {
		t.Fatalf("ListenAddress = %q, want configured value", cfg.ListenAddress)
	}
	if cfg.AppEncryptionKey != "test-encryption-key" {
		t.Fatalf("AppEncryptionKey = %q, want configured value", cfg.AppEncryptionKey)
	}
}

func TestLoadWithoutVAPIDKeysStillStarts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("APP_ENCRYPTION_KEY", "test-encryption-key")
	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")
	t.Setenv("VAPID_SUBJECT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want push notifications to be optional", err)
	}
	if cfg.PushPublicKey != "" || cfg.PushPrivateKey != "" {
		t.Fatal("expected empty push keys when VAPID env vars are unset")
	}
}

func TestLoadDefaultsCookieSecureToTrue(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("APP_ENCRYPTION_KEY", "test-encryption-key")
	t.Setenv("VAPID_PUBLIC_KEY", "test-public-key")
	t.Setenv("VAPID_PRIVATE_KEY", "test-private-key")
	t.Setenv("VAPID_SUBJECT", "mailto:test@example.com")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.CookieSecure {
		t.Fatal("CookieSecure = false, want true by default")
	}
}
