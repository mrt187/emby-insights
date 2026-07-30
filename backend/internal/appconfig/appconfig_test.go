package appconfig

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrt187/EmbyInsights/internal/secretbox"
	"github.com/mrt187/EmbyInsights/internal/store"
)

// These tests exercise the atomic first-admin claim and settings persistence
// against a real Postgres instance, matching how the rest of the app relies
// on the database's own guarantees rather than mocking it. They need
// TEST_DATABASE_URL (or DATABASE_URL) pointed at a disposable database and
// are skipped otherwise, e.g. in sandboxes without Postgres available.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL (or DATABASE_URL) to run appconfig tests against a real Postgres instance")
	}

	ctx := context.Background()
	if err := store.Migrate(ctx, databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, "TRUNCATE admin_owner, app_config"); err != nil {
		t.Fatalf("truncate setup-wizard tables: %v", err)
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("generate test encryption key: %v", err)
	}
	box, err := secretbox.New(base64.StdEncoding.EncodeToString(keyBytes))
	if err != nil {
		t.Fatalf("secretbox.New() error = %v", err)
	}

	return NewStore(pool, box)
}

func TestClaimAdminOwnerIsAtomicUnderConcurrency(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	const candidates = 12
	results := make([]string, candidates)
	errs := make([]error, candidates)
	var wg sync.WaitGroup
	for i := 0; i < candidates; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			owner, err := appStore.ClaimAdminOwner(ctx, "user-"+string(rune('a'+index)))
			results[index] = owner
			errs[index] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("ClaimAdminOwner()[%d] error = %v", i, err)
		}
	}
	winner := results[0]
	if winner == "" {
		t.Fatal("ClaimAdminOwner() produced no winner")
	}
	for i, result := range results {
		if result != winner {
			t.Fatalf("ClaimAdminOwner()[%d] = %q, want everyone to agree on winner %q", i, result, winner)
		}
	}
}

func TestClaimAdminOwnerKeepsFirstWinner(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	first, err := appStore.ClaimAdminOwner(ctx, "first-user")
	if err != nil {
		t.Fatalf("ClaimAdminOwner() error = %v", err)
	}
	if first != "first-user" {
		t.Fatalf("ClaimAdminOwner() = %q, want %q", first, "first-user")
	}

	second, err := appStore.ClaimAdminOwner(ctx, "second-user")
	if err != nil {
		t.Fatalf("ClaimAdminOwner() error = %v", err)
	}
	if second != "first-user" {
		t.Fatalf("ClaimAdminOwner() second call = %q, want it to keep the original winner %q", second, "first-user")
	}
}

func TestEnsureDeviceIDGeneratesOnceAndPersists(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	first, err := appStore.EnsureDeviceID(ctx)
	if err != nil {
		t.Fatalf("EnsureDeviceID() error = %v", err)
	}
	if first == "" {
		t.Fatal("EnsureDeviceID() returned empty id")
	}

	second, err := appStore.EnsureDeviceID(ctx)
	if err != nil {
		t.Fatalf("EnsureDeviceID() error = %v", err)
	}
	if second != first {
		t.Fatalf("EnsureDeviceID() second call = %q, want it to keep %q", second, first)
	}
}

func TestUpdateSettingsKeepsExistingKeyWhenFieldEmpty(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	if err := appStore.Update(ctx, Settings{
		Seerr:               ServiceSetting{Enabled: true, BaseURL: "http://seerr.local", APIKey: "initial-key"},
		ComingSoonRegion:    "DE",
		ComingSoonDaysAhead: 28,
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Second update leaves APIKey empty (as the admin UI does when the
	// operator didn't type a new one) but changes the base URL.
	if err := appStore.Update(ctx, Settings{
		Seerr:               ServiceSetting{Enabled: true, BaseURL: "http://seerr.new", APIKey: ""},
		ComingSoonRegion:    "DE",
		ComingSoonDaysAhead: 28,
	}); err != nil {
		t.Fatalf("Update() second call error = %v", err)
	}

	settings, err := appStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settings.Seerr.BaseURL != "http://seerr.new" {
		t.Fatalf("Seerr.BaseURL = %q, want updated URL", settings.Seerr.BaseURL)
	}
	if settings.Seerr.APIKey != "initial-key" {
		t.Fatalf("Seerr.APIKey = %q, want the original key to be preserved", settings.Seerr.APIKey)
	}
}
