package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
}

func TestLoadUsesConfiguredValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("LISTEN_ADDRESS", "127.0.0.1:9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:9090" {
		t.Fatalf("ListenAddress = %q, want configured value", cfg.ListenAddress)
	}
}
