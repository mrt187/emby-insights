package store

import (
	"context"
	"errors"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MediaTracking is a user's personal note on one title — a 1-5 star rating,
// watchlist membership, or both. MediaSource/MediaID mirror the frontend's
// MediaSelection type, so a title Seerr knows about but Emby doesn't yet can
// still go on the watchlist.
type MediaTracking struct {
	MediaSource      string `json:"mediaSource"`
	MediaID          string `json:"mediaId"`
	MediaType        string `json:"mediaType"`
	Title            string `json:"title"`
	PosterURL        string `json:"posterUrl"`
	Rating           int    `json:"rating,omitempty"`
	OnWatchlist      bool   `json:"onWatchlist"`
	HiddenInProgress bool   `json:"hiddenInProgress,omitempty"`
	// PosterImageData/PosterImageContentType carry a freshly fetched poster
	// to persist alongside the rating (see server.fetchPosterBytes) — never
	// part of the JSON API, only used internally by Upsert.
	PosterImageData        []byte `json:"-"`
	PosterImageContentType string `json:"-"`
}

type TrackingStore interface {
	Get(ctx context.Context, embyUserID, mediaSource, mediaID string) (MediaTracking, bool, error)
	Upsert(ctx context.Context, embyUserID string, entry MediaTracking) error
	Watchlist(ctx context.Context, embyUserID string) ([]MediaTracking, error)
	Ratings(ctx context.Context, embyUserID string) ([]MediaTracking, error)
	HiddenInProgressIDs(ctx context.Context, embyUserID string) (map[string]bool, error)
	TopRatings(ctx context.Context, limit int) ([]AggregatedRating, error)
	PosterImage(ctx context.Context, mediaSource, mediaID string) ([]byte, string, bool, error)
}

// AggregatedRating is one title's rating averaged across every Emby Insights
// user who rated it — the "Top Bewertet" home row, household taste rather
// than Emby's own community/critics score.
type AggregatedRating struct {
	ID            string  `json:"id"`
	MediaSource   string  `json:"mediaSource"`
	MediaID       string  `json:"mediaId"`
	MediaType     string  `json:"mediaType"`
	Title         string  `json:"title"`
	PosterURL     string  `json:"posterUrl"`
	AverageRating float64 `json:"averageRating"`
}

type PostgresTrackingStore struct {
	pool *pgxpool.Pool
}

func NewPostgresTrackingStore(pool *pgxpool.Pool) *PostgresTrackingStore {
	return &PostgresTrackingStore{pool: pool}
}

// posterServingURL points the browser at our own permanently-cached poster
// once one has been stored, instead of the (possibly stale) URL captured at
// rating time — see migration 009.
func posterServingURL(mediaSource, mediaID string, hasImage bool, fallback string) string {
	if !hasImage {
		return fallback
	}
	return "/api/tracking/poster?source=" + url.QueryEscape(mediaSource) + "&id=" + url.QueryEscape(mediaID)
}

// Get returns the tracking entry for one title, and false if the user has
// never rated, watchlisted or hidden it.
func (store *PostgresTrackingStore) Get(ctx context.Context, embyUserID, mediaSource, mediaID string) (MediaTracking, bool, error) {
	row := store.pool.QueryRow(ctx, `
		SELECT media_source, media_id, media_type, title, poster_url, poster_image IS NOT NULL, COALESCE(rating, 0), on_watchlist, hidden_in_progress
		FROM media_tracking
		WHERE emby_user_id = $1 AND media_source = $2 AND media_id = $3
	`, embyUserID, mediaSource, mediaID)

	var entry MediaTracking
	var hasImage bool
	err := row.Scan(&entry.MediaSource, &entry.MediaID, &entry.MediaType, &entry.Title, &entry.PosterURL, &hasImage,
		&entry.Rating, &entry.OnWatchlist, &entry.HiddenInProgress)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaTracking{}, false, nil
	}
	if err != nil {
		return MediaTracking{}, false, err
	}
	entry.PosterURL = posterServingURL(entry.MediaSource, entry.MediaID, hasImage, entry.PosterURL)
	return entry, true, nil
}

// Upsert stores a rating, watchlist membership and/or hidden-in-progress
// flag for one title. A rating of 0 clears it (NULLIF turns it into SQL
// NULL) rather than storing an invalid value. PosterImageData is only
// overwritten when the caller actually fetched a fresh one (see
// server.upsertTracking) — an empty value leaves whatever was already
// stored untouched, so a failed fetch on one save doesn't wipe out a poster
// a previous save already cached.
func (store *PostgresTrackingStore) Upsert(ctx context.Context, embyUserID string, entry MediaTracking) error {
	var posterContentType *string
	if entry.PosterImageContentType != "" {
		posterContentType = &entry.PosterImageContentType
	}
	_, err := store.pool.Exec(ctx, `
		INSERT INTO media_tracking
			(emby_user_id, media_source, media_id, media_type, title, poster_url, poster_image, poster_content_type, rating, on_watchlist, hidden_in_progress, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, 0), $10, $11, CURRENT_TIMESTAMP)
		ON CONFLICT (emby_user_id, media_source, media_id) DO UPDATE SET
			media_type          = EXCLUDED.media_type,
			title               = EXCLUDED.title,
			poster_url          = EXCLUDED.poster_url,
			poster_image         = COALESCE(EXCLUDED.poster_image, media_tracking.poster_image),
			poster_content_type = COALESCE(EXCLUDED.poster_content_type, media_tracking.poster_content_type),
			rating              = EXCLUDED.rating,
			on_watchlist        = EXCLUDED.on_watchlist,
			hidden_in_progress  = EXCLUDED.hidden_in_progress,
			updated_at          = CURRENT_TIMESTAMP
	`, embyUserID, entry.MediaSource, entry.MediaID, entry.MediaType, entry.Title, entry.PosterURL,
		entry.PosterImageData, posterContentType, entry.Rating, entry.OnWatchlist, entry.HiddenInProgress)
	return err
}

func (store *PostgresTrackingStore) Watchlist(ctx context.Context, embyUserID string) ([]MediaTracking, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT media_source, media_id, media_type, title, poster_url, poster_image IS NOT NULL, COALESCE(rating, 0), on_watchlist, hidden_in_progress
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
		SELECT media_source, media_id, media_type, title, poster_url, poster_image IS NOT NULL, COALESCE(rating, 0), on_watchlist, hidden_in_progress
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

// TopRatings averages every user's rating per title across the whole
// household and returns the highest-rated ones, most-agreed-on first when
// averages tie.
func (store *PostgresTrackingStore) TopRatings(ctx context.Context, limit int) ([]AggregatedRating, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT media_source, media_id, MAX(media_type), MAX(title), MAX(poster_url), BOOL_OR(poster_image IS NOT NULL), AVG(rating)::float8, COUNT(*)
		FROM media_tracking
		WHERE rating IS NOT NULL
		GROUP BY media_source, media_id
		ORDER BY AVG(rating) DESC, COUNT(*) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AggregatedRating
	for rows.Next() {
		var entry AggregatedRating
		var hasImage bool
		var count int
		if err := rows.Scan(&entry.MediaSource, &entry.MediaID, &entry.MediaType, &entry.Title, &entry.PosterURL, &hasImage, &entry.AverageRating, &count); err != nil {
			return nil, err
		}
		entry.ID = entry.MediaSource + ":" + entry.MediaID
		entry.PosterURL = posterServingURL(entry.MediaSource, entry.MediaID, hasImage, entry.PosterURL)
		results = append(results, entry)
	}
	return results, rows.Err()
}

// PosterImage returns the cached poster bytes for one title, across whoever
// rated it first — used by the /api/tracking/poster handler.
func (store *PostgresTrackingStore) PosterImage(ctx context.Context, mediaSource, mediaID string) ([]byte, string, bool, error) {
	row := store.pool.QueryRow(ctx, `
		SELECT poster_image, poster_content_type
		FROM media_tracking
		WHERE media_source = $1 AND media_id = $2 AND poster_image IS NOT NULL
		LIMIT 1
	`, mediaSource, mediaID)

	var data []byte
	var contentType string
	err := row.Scan(&data, &contentType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	return data, contentType, true, nil
}

// HiddenInProgressIDs returns the Emby item ids a user dismissed from "Noch
// nicht fertig", so the handler can filter them out of the Emby-sourced
// series list without a per-item round trip.
func (store *PostgresTrackingStore) HiddenInProgressIDs(ctx context.Context, embyUserID string) (map[string]bool, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT media_id FROM media_tracking
		WHERE emby_user_id = $1 AND media_source = 'emby' AND hidden_in_progress
	`, embyUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var mediaID string
		if err := rows.Scan(&mediaID); err != nil {
			return nil, err
		}
		ids[mediaID] = true
	}
	return ids, rows.Err()
}

func scanTracking(rows pgx.Rows) ([]MediaTracking, error) {
	var results []MediaTracking
	for rows.Next() {
		var entry MediaTracking
		var hasImage bool
		if err := rows.Scan(&entry.MediaSource, &entry.MediaID, &entry.MediaType, &entry.Title, &entry.PosterURL, &hasImage,
			&entry.Rating, &entry.OnWatchlist, &entry.HiddenInProgress); err != nil {
			return nil, err
		}
		entry.PosterURL = posterServingURL(entry.MediaSource, entry.MediaID, hasImage, entry.PosterURL)
		results = append(results, entry)
	}
	return results, rows.Err()
}
