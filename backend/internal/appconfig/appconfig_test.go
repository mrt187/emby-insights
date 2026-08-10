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

// A regenerated public key silently kills push for every existing browser
// subscription (see push.NewSender's doc comment), so a save that doesn't
// touch push at all — e.g. only flipping Seerr's URL — must never clear or
// change an already-stored keypair.
func TestUpdateSettingsKeepsPushKeypairWhenNotProvided(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	if err := appStore.Update(ctx, Settings{
		ComingSoonRegion:    "DE",
		ComingSoonDaysAhead: 28,
		Push:                PushSetting{Enabled: true, Subject: "mailto:admin@example.com", PublicKey: "initial-public", PrivateKey: "initial-private"},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Second update is an unrelated Seerr-only change, as the admin UI sends
	// when the operator never touched the push section — PublicKey/PrivateKey
	// are zero-value, not resent.
	if err := appStore.Update(ctx, Settings{
		Seerr:               ServiceSetting{Enabled: true, BaseURL: "http://seerr.local", APIKey: "seerr-key"},
		ComingSoonRegion:    "DE",
		ComingSoonDaysAhead: 28,
		Push:                PushSetting{Enabled: true, Subject: "mailto:admin@example.com"},
	}); err != nil {
		t.Fatalf("Update() second call error = %v", err)
	}

	settings, err := appStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settings.Push.PublicKey != "initial-public" || settings.Push.PrivateKey != "initial-private" {
		t.Fatalf("Push keypair = %+v, want the original keypair to be preserved", settings.Push)
	}
}

// Existing installs that configured push the old way (VAPID_* in .env)
// must keep the exact same keypair once it moves into app_config — a
// different key breaks every already-registered browser subscription.
func TestSeedPushFromEnvIfEmptyCarriesOverExistingKeypair(t *testing.T) {
	appStore := newTestStore(t)
	ctx := context.Background()

	t.Setenv("VAPID_PUBLIC_KEY", "env-public-key")
	t.Setenv("VAPID_PRIVATE_KEY", "env-private-key")
	t.Setenv("VAPID_SUBJECT", "mailto:ops@example.com")

	if err := appStore.SeedPushFromEnvIfEmpty(ctx); err != nil {
		t.Fatalf("SeedPushFromEnvIfEmpty() error = %v", err)
	}

	settings, err := appStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !settings.Push.Enabled || settings.Push.PublicKey != "env-public-key" || settings.Push.PrivateKey != "env-private-key" || settings.Push.Subject != "mailto:ops@example.com" {
		t.Fatalf("Push = %+v, want the env keypair seeded verbatim", settings.Push)
	}

	// Running it again (e.g. every container start) must never touch an
	// already-seeded keypair, even if the env still has the same values.
	t.Setenv("VAPID_PUBLIC_KEY", "different-public-key")
	if err := appStore.SeedPushFromEnvIfEmpty(ctx); err != nil {
		t.Fatalf("SeedPushFromEnvIfEmpty() second call error = %v", err)
	}
	settings, err = appStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settings.Push.PublicKey != "env-public-key" {
		t.Fatalf("Push.PublicKey = %q after re-seed, want the original key to stick", settings.Push.PublicKey)
	}
}
