package server

import (
	"context"
	"log"
)

// BackfillPosterImages fetches and persists poster bytes for every rated
// title that predates migration 009 (poster_image/poster_content_type),
// so already-broken "Top Bewertet" posters recover without the user having
// to re-rate anything. Runs once at startup, best-effort per title — one
// title that's since vanished from Emby must not stop the rest.
//
// Emby-sourced titles need a fresh image tag resolved via a full detail
// lookup first (the tag stored in poster_url from rating time is exactly
// what went stale); everything else already has a stable absolute URL
// (Seerr/TMDB) and is fetched directly.
func (app *App) BackfillPosterImages(ctx context.Context) {
	rows, err := app.database.Query(ctx, `
		SELECT DISTINCT ON (media_source, media_id) media_source, media_id, emby_user_id, poster_url
		FROM media_tracking
		WHERE poster_image IS NULL AND poster_url IS NOT NULL AND poster_url <> ''
		ORDER BY media_source, media_id
	`)
	if err != nil {
		log.Printf("poster backfill: query candidates: %v", err)
		return
	}
	type candidate struct {
		mediaSource, mediaID, embyUserID, posterURL string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.mediaSource, &c.mediaID, &c.embyUserID, &c.posterURL); err != nil {
			log.Printf("poster backfill: scan candidate: %v", err)
			continue
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("poster backfill: read candidates: %v", err)
		return
	}
	if len(candidates) == 0 {
		return
	}

	fixed := 0
	for _, c := range candidates {
		posterURL := c.posterURL
		if c.mediaSource == "emby" {
			detail, err := app.embyMediaDetail.EmbyMediaDetail(ctx, c.embyUserID, c.mediaID)
			if err != nil || detail.PosterURL == "" {
				continue
			}
			posterURL = detail.PosterURL
		}
		data, contentType, ok := app.fetchPosterBytes(ctx, c.mediaSource, posterURL)
		if !ok {
			continue
		}
		if _, err := app.database.Exec(ctx, `
			UPDATE media_tracking SET poster_image = $1, poster_content_type = $2
			WHERE media_source = $3 AND media_id = $4 AND poster_image IS NULL
		`, data, contentType, c.mediaSource, c.mediaID); err != nil {
			log.Printf("poster backfill: store %s:%s: %v", c.mediaSource, c.mediaID, err)
			continue
		}
		fixed++
	}
	log.Printf("poster backfill: cached %d/%d titles", fixed, len(candidates))
}
