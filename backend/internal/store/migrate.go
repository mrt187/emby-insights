package store

import (
	"context"
	"embed"
	"fmt"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID is an arbitrary constant advisory-lock key, unique to
// Emby Insights, so that overlapping containers during an Unraid "Update
// Stack" never apply the same migration twice.
const migrationLockID = 8743201958

type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies every embedded migration whose version is higher than the
// database's recorded schema_migrations version. It opens its own short-lived
// connection rather than the app's pool, since pgxpool.New never actually
// connects (it is lazy) and this must run before anything else touches the
// database.
func Migrate(ctx context.Context, databaseURL string) error {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(migrationLockID)); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", int64(migrationLockID))

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	var currentVersion int
	if err := conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion); err != nil {
		return fmt.Errorf("read current schema version: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	highestKnown := 0
	for _, candidate := range migrations {
		if candidate.version > highestKnown {
			highestKnown = candidate.version
		}
		if candidate.version <= currentVersion {
			continue
		}
		if err := applyMigration(ctx, conn, candidate); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", candidate.version, candidate.name, err)
		}
		log.Printf("applied migration %d (%s)", candidate.version, candidate.name)
	}

	// A rollback to an older image must not abort startup — it should still
	// serve traffic against whatever schema is actually there.
	if currentVersion > highestKnown {
		log.Printf("warning: database schema version %d is newer than the highest known migration %d", currentVersion, highestKnown)
	}

	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, err := versionFromFilename(entry.Name())
		if err != nil {
			return nil, err
		}
		content, err := migrationFiles.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{version: version, name: entry.Name(), sql: string(content)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

func versionFromFilename(name string) (int, error) {
	prefix, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("migration filename %q must start with a numeric version followed by '_'", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("migration filename %q has a non-numeric version prefix: %w", name, err)
	}
	return version, nil
}

func applyMigration(ctx context.Context, conn *pgx.Conn, candidate migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, candidate.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", candidate.version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
