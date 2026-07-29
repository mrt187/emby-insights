package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MediaTracking is a user's personal note on one title — a 1-5 star rating,
// watchlist membership, or both. MediaSource/MediaID mirror the frontend's
// MediaSelection type, so a title Seerr knows about but Emby doesn't yet can
// still go on the watchlist.
type MediaTracking struct {
	MediaSource  string `json:"mediaSource"`
	MediaID      string `json:"mediaId"`
	MediaType    string `json:"mediaType"`
	Title        string `json:"title"`
	PosterURL    string `json:"posterUrl"`
	Rating       int    `json:"rating,omitempty"`
	OnWatchlist  bool   `json:"onWatchlist"`
	RewatchCount int    `json:"rewatchCount"`
}

type TrackingStore interface {
	Get(ctx context.Context, embyUserID, mediaSource, mediaID string) (MediaTracking, bool, error)
	Upsert(ctx context.Context, embyUserID string, entry MediaTracking) error
	Watchlist(ctx context.Context, embyUserID string) ([]MediaTracking, error)
	Ratings(ctx context.Context, embyUserID string) ([]MediaTracking, error)
}

type PostgresTrackingStore struct {
	pool *pgxpool.Pool
}

func NewPostgresTrackingStore(pool *pgxpool.Pool) *PostgresTrackingStore {
	return &PostgresTrackingStore{pool: pool}
}

// Get returns the tracking entry for one title, and false if the user has
// never rated or watchlisted it.
func (store *PostgresTrackingStore) Get(ctx context.Context, embyUserID, mediaSource, mediaID string) (MediaTracking, bool, error) {
	row := store.pool.QueryRow(ctx, `
		SELECT media_source, media_id, media_type, title, poster_url, COALESCE(rating, 0), on_watchlist, rewatch_count
		FROM media_tracking
		WHERE emby_user_id = $1 AND media_source = $2 AND media_id = $3
	`, embyUserID, mediaSource, mediaID)

	var entry MediaTracking
	err := row.Scan(&entry.MediaSource, &entry.MediaID, &entry.MediaType, &entry.Title, &entry.PosterURL,
		&entry.Rating, &entry.OnWatchlist, &entry.RewatchCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaTracking{}, false, nil
	}
	if err != nil {
		return MediaTracking{}, false, err
	}
	return entry, true, nil
}

// Upsert stores a rating and/or watchlist membership for one title. A rating
// of 0 clears it (NULLIF turns it into SQL NULL) rather than storing an
// invalid value.
func (store *PostgresTrackingStore) Upsert(ctx context.Context, embyUserID string, entry MediaTracking) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO media_tracking
			(emby_user_id, media_source, media_id, media_type, title, poster_url, rating, on_watchlist, rewatch_count, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, NULLIF($7, 0), $8, $9, CURRENT_TIMESTAMP)
		ON CONFLICT (emby_user_id, media_source, media_id) DO UPDATE SET
			media_type    = EXCLUDED.media_type,
			title         = EXCLUDED.title,
			poster_url    = EXCLUDED.poster_url,
			rating        = EXCLUDED.rating,
			on_watchlist  = EXCLUDED.on_watchlist,
			rewatch_count = EXCLUDED.rewatch_count,
			updated_at    = CURRENT_TIMESTAMP
	`, embyUserID, entry.MediaSource, entry.MediaID, entry.MediaType, entry.Title, entry.PosterURL,
		entry.Rating, entry.OnWatchlist, entry.RewatchCount)
	return err
}

func (store *PostgresTrackingStore) Watchlist(ctx context.Context, embyUserID string) ([]MediaTracking, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT media_source, media_id, media_type, title, poster_url, COALESCE(rating, 0), on_watchlist, rewatch_count
		FROM media_tracking
		WHERE emby_user_id = $1 AND on_watchlist
		ORDER BY updated_at DESC
	`, embyUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracking(rows)
}

func (store *PostgresTrackingStore) Ratings(ctx context.Context, embyUserID string) ([]MediaTracking, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT media_source, media_id, media_type, title, poster_url, COALESCE(rating, 0), on_watchlist, rewatch_count
		FROM media_tracking
		WHERE emby_user_id = $1 AND rating IS NOT NULL
		ORDER BY updated_at DESC
	`, embyUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracking(rows)
}

func scanTracking(rows pgx.Rows) ([]MediaTracking, error) {
	var results []MediaTracking
	for rows.Next() {
		var entry MediaTracking
		if err := rows.Scan(&entry.MediaSource, &entry.MediaID, &entry.MediaType, &entry.Title, &entry.PosterURL,
			&entry.Rating, &entry.OnWatchlist, &entry.RewatchCount); err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}
