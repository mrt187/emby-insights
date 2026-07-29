package store

import "testing"

func TestLoadMigrationsSortedByVersion(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) < 2 {
		t.Fatalf("expected at least 2 embedded migrations, got %d", len(migrations))
	}
	for i := 1; i < len(migrations); i++ {
		if migrations[i].version <= migrations[i-1].version {
			t.Fatalf("migrations not sorted ascending: %v", migrations)
		}
	}
	if migrations[0].version != 1 {
		t.Fatalf("first migration version = %d, want 1", migrations[0].version)
	}
}

func TestVersionFromFilename(t *testing.T) {
	cases := map[string]int{
		"001_initial_schema.sql": 1,
		"002_media_tracking.sql": 2,
		"42_something.sql":       42,
	}
	for name, want := range cases {
		got, err := versionFromFilename(name)
		if err != nil {
			t.Fatalf("versionFromFilename(%q) error = %v", name, err)
		}
		if got != want {
			t.Fatalf("versionFromFilename(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestVersionFromFilenameRejectsMissingSeparator(t *testing.T) {
	if _, err := versionFromFilename("schema.sql"); err == nil {
		t.Fatal("expected an error for a filename without a version prefix")
	}
}
