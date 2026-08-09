package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/mrt187/EmbyInsights/internal/tracearr"
)

// identityCacheTTL is deliberately long: mapping an Emby user id to a
// Tracearr identity costs a full walk of /users, and that mapping only
// changes when an account is added to or removed from the server.
const tracearrIdentityCacheTTL = 24 * time.Hour

// tracearrIdentity resolves the signed-in Emby user to their Tracearr
// identity, or "" when Tracearr is off, unreachable, or does not know this
// account. Callers treat "" as "no Tracearr data", never as an error.
func (app *App) tracearrIdentity(ctx context.Context, embyUserID string) (*tracearr.Client, string) {
	client := app.live.tracearrClient()
	if client == nil {
		return nil, ""
	}
	identityID, err := cachedJSON(ctx, app, "tracearr:identity:"+embyUserID, tracearrIdentityCacheTTL, func(ctx context.Context) (string, error) {
		return client.IdentityID(ctx, embyUserID)
	})
	if err != nil || identityID == "" {
		return nil, ""
	}
	return client, identityID
}

// periodStart converts the period values the statistics screen already uses
// into the "since" instant Tracearr's history filter expects.
func periodStart(period string) time.Time {
	now := time.Now().UTC()
	switch period {
	case "month":
		return now.AddDate(0, -1, 0)
	case "year":
		return now.AddDate(-1, 0, 0)
	default:
		return now.AddDate(0, 0, -7)
	}
}

// genreStats serves the full genre breakdown. Playback Reporting only
// yields a single favourite genre; Tracearr counts genres per play, so this
// is the one place the two sources genuinely differ rather than overlap.
func (app *App) genreStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	client, identityID := app.tracearrIdentity(request.Context(), identity.UserID)
	if client == nil {
		respondJSON(writer, http.StatusOK, []tracearr.Genre{})
		return
	}
	genres, err := cachedJSON(request.Context(), app, "tracearr:genres:"+identityID, statsCacheTTL, func(ctx context.Context) ([]tracearr.Genre, error) {
		return client.TopGenres(ctx, identityID)
	})
	if err != nil {
		respondJSON(writer, http.StatusOK, []tracearr.Genre{})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(genres))
}

func (app *App) unfinishedStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	client, identityID := app.tracearrIdentity(request.Context(), identity.UserID)
	if client == nil {
		respondJSON(writer, http.StatusOK, []tracearr.UnfinishedPlay{})
		return
	}
	plays, err := cachedJSON(request.Context(), app, "tracearr:unfinished:"+identityID+":"+period, statsCacheTTL, func(ctx context.Context) ([]tracearr.UnfinishedPlay, error) {
		return client.Unfinished(ctx, identityID, periodStart(period))
	})
	if err != nil {
		respondJSON(writer, http.StatusOK, []tracearr.UnfinishedPlay{})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(plays))
}

func (app *App) transcodeShareStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	client, identityID := app.tracearrIdentity(request.Context(), identity.UserID)
	if client == nil {
		respondJSON(writer, http.StatusOK, tracearr.TranscodeShare{})
		return
	}
	share, err := cachedJSON(request.Context(), app, "tracearr:transcode:"+identityID+":"+period, statsCacheTTL, func(ctx context.Context) (tracearr.TranscodeShare, error) {
		return client.TranscodeShare(ctx, identityID, periodStart(period))
	})
	if err != nil {
		respondJSON(writer, http.StatusOK, tracearr.TranscodeShare{})
		return
	}
	respondJSON(writer, http.StatusOK, share)
}

// household is the "who else here watched this, and how often" block on a
// media-detail screen. It is decoration on an otherwise complete response,
// so like the OMDb merge it swallows every failure.
type household struct {
	Plays       int                `json:"plays"`
	UniqueUsers int                `json:"uniqueUsers"`
	Watchers    []tracearr.Watcher `json:"watchers"`
}

// tracearrHousehold looks up household activity for one title. mediaType is
// Emby's own item type ("Movie"/"Series"); the ids are whichever external
// ids the caller has. It returns nil whenever Tracearr is off or the title
// cannot be addressed, and the caller then simply omits the block.
func (app *App) tracearrHousehold(ctx context.Context, embyMediaType, imdbID, tmdbID, tvdbID string) *household {
	client := app.live.tracearrClient()
	if client == nil {
		return nil
	}
	mediaType := tracearr.EmbyMediaType(embyMediaType)
	if mediaType == "" {
		return nil
	}

	// Provider preference follows how reliably each id identifies a title in
	// Tracearr's own matching: TMDB is what both Radarr/Sonarr and Seerr key
	// on here, IMDb is the most universal, TVDB only ever applies to shows.
	var ref string
	for _, candidate := range []struct{ provider, id string }{
		{"tmdb", tmdbID},
		{"imdb", imdbID},
		{"tvdb", tvdbID},
	} {
		if ref = tracearr.Ref(mediaType, candidate.provider, candidate.id); ref != "" {
			break
		}
	}
	if ref == "" {
		return nil
	}

	result, err := cachedJSON(ctx, app, "tracearr:household:"+ref, statsCacheTTL, func(ctx context.Context) (household, error) {
		stats, err := client.MediaStats(ctx, ref)
		if err != nil {
			return household{}, err
		}
		watchers, err := client.Watchers(ctx, ref)
		if err != nil {
			return household{}, err
		}
		return household{Plays: stats.Plays, UniqueUsers: stats.UniqueUsers, Watchers: watchers}, nil
	})
	if err != nil || len(result.Watchers) == 0 {
		return nil
	}
	return &result
}

// tracearrHouseholdForTMDB is tracearrHousehold for the Seerr detail screen,
// which knows a title only by TMDB id and Seerr's "movie"/"tv" vocabulary.
func (app *App) tracearrHouseholdForTMDB(ctx context.Context, seerrMediaType string, tmdbID int, imdbID string) *household {
	embyMediaType := "Movie"
	if seerrMediaType == "tv" {
		embyMediaType = "Series"
	}
	return app.tracearrHousehold(ctx, embyMediaType, imdbID, strconv.Itoa(tmdbID), "")
}
