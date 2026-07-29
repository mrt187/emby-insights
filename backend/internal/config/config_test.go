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
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_DEVICE_ID", "test-device")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:9090" {
		t.Fatalf("ListenAddress = %q, want configured value", cfg.ListenAddress)
	}
}

func TestLoadLeavesSeerrAndComingSoonUnconfiguredByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_DEVICE_ID", "test-device")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SeerrBaseURL != "" || cfg.SeerrAPIKey != "" || cfg.EmbyComingSoonLibraryIDs != nil {
		t.Fatalf("cfg = %#v, want Seerr and ComingSoon left unconfigured", cfg)
	}
}

func TestLoadParsesComingSoonLibraryIDs(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/emby_insights")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("EMBY_BASE_URL", "http://emby:8096/emby")
	t.Setenv("EMBY_DEVICE_ID", "test-device")
	t.Setenv("EMBY_ADMIN_API_KEY", "test-admin-key")
	t.Setenv("EMBY_COMINGSOON_LIBRARY_IDS", " library-1 ,library-2,")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"library-1", "library-2"}
	if len(cfg.EmbyComingSoonLibraryIDs) != len(want) || cfg.EmbyComingSoonLibraryIDs[0] != want[0] || cfg.EmbyComingSoonLibraryIDs[1] != want[1] {
		t.Fatalf("EmbyComingSoonLibraryIDs = %#v, want %#v", cfg.EmbyComingSoonLibraryIDs, want)
	}
}
