package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DailyActivity is one day's worth of the Verwaltung activity chart: how
// many Seerr requests Emby Insights actually triggered, and how many
// distinct users touched the app at all that day.
type DailyActivity struct {
	Date         string `json:"date"` // YYYY-MM-DD
	RequestCount int    `json:"requestCount"`
	ActiveUsers  int    `json:"activeUsers"`
}

type ActivityStore interface {
	RecordActive(ctx context.Context, embyUserID string, day time.Time) error
	RecordSeerrRequest(ctx context.Context, embyUserID, mediaType string, tmdbID int) error
	WeeklyActivity(ctx context.Context, days int) ([]DailyActivity, error)
}

type PostgresActivityStore struct {
	pool *pgxpool.Pool
}

func NewPostgresActivityStore(pool *pgxpool.Pool) *PostgresActivityStore {
	return &PostgresActivityStore{pool: pool}
}

// RecordActive marks a user active on the given day. The caller is expected
// to dedupe repeat calls for the same user/day (via Redis) before reaching
// here — ON CONFLICT DO NOTHING is just a safety net, not the dedup strategy,
// since every call still costs a write attempt.
func (store *PostgresActivityStore) RecordActive(ctx context.Context, embyUserID string, day time.Time) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO daily_activity (emby_user_id, activity_date)
		VALUES ($1, $2)
		ON CONFLICT (emby_user_id, activity_date) DO NOTHING
	`, embyUserID, day.Format("2006-01-02"))
	return err
}

func (store *PostgresActivityStore) RecordSeerrRequest(ctx context.Context, embyUserID, mediaType string, tmdbID int) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO seerr_request_log (emby_user_id, media_type, tmdb_id)
		VALUES ($1, $2, $3)
	`, embyUserID, mediaType, tmdbID)
	return err
}

// WeeklyActivity returns exactly `days` rows, oldest first, one per
// calendar day up to and including today — days with no activity at all
// still appear, with zero counts, so the chart never has gaps.
func (store *PostgresActivityStore) WeeklyActivity(ctx context.Context, days int) ([]DailyActivity, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT
			d::date AS day,
			COALESCE(r.request_count, 0) AS request_count,
			COALESCE(a.active_users, 0) AS active_users
		FROM generate_series(CURRENT_DATE - ($1::int - 1), CURRENT_DATE, interval '1 day') AS d
		LEFT JOIN (
			SELECT date_trunc('day', created_at)::date AS day, COUNT(*) AS request_count
			FROM seerr_request_log
			WHERE created_at >= CURRENT_DATE - ($1::int - 1)
			GROUP BY 1
		) r ON r.day = d::date
		LEFT JOIN (
			SELECT activity_date AS day, COUNT(DISTINCT emby_user_id) AS active_users
			FROM daily_activity
			WHERE activity_date >= CURRENT_DATE - ($1::int - 1)
			GROUP BY 1
		) a ON a.day = d::date
		ORDER BY d
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DailyActivity
	for rows.Next() {
		var entry DailyActivity
		var day time.Time
		if err := rows.Scan(&day, &entry.RequestCount, &entry.ActiveUsers); err != nil {
			return nil, err
		}
		entry.Date = day.Format("2006-01-02")
		results = append(results, entry)
	}
	return results, rows.Err()
}
