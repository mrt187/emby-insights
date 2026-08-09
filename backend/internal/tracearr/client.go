// Package tracearr reads playback insights from a self-hosted Tracearr
// instance's public v2 API.
//
// Tracearr is not a replacement for the Emby Insights connector plugin: the
// plugin remains the source for watch time, devices, hours and weekdays,
// which it derives from Playback Reporting. Tracearr records things
// Playback Reporting never captures — genres per play, how far a play
// actually got, resume chains, who else in the household watched a title,
// and whether a stream was transcoded — so this package covers only that
// gap. Everything it returns is decoration on top of an already-complete
// page.
package tracearr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxPages caps cursor pagination. A household account with years of
// history would otherwise turn one dashboard request into an unbounded
// series of upstream calls; the features here only ever need the most
// recent plays.
const maxPages = 10

// maxHouseholdPages is the higher cap for the most-watched list. That one
// reads everyone's plays rather than one person's, so 30 days of a busy
// household easily exceeds maxPages*pageSize records — and a truncated feed
// would silently drop titles out of the ranking rather than fail loudly.
const maxHouseholdPages = 30

const pageSize = 100

type Genre struct {
	Genre string `json:"genre"`
	Plays int    `json:"plays"`
}

// UnfinishedPlay is a title the user started and never completed —
// Tracearr's `watched` flag is false and playback stopped somewhere in the
// middle rather than at the very start.
type UnfinishedPlay struct {
	MediaID         string  `json:"mediaId"`
	MediaType       string  `json:"mediaType"`
	Title           string  `json:"title"`
	ShowTitle       string  `json:"showTitle,omitempty"`
	SeasonNumber    int     `json:"seasonNumber,omitempty"`
	EpisodeNumber   int     `json:"episodeNumber,omitempty"`
	Year            int     `json:"year,omitempty"`
	PercentComplete float64 `json:"percentComplete"`
	StoppedAt       string  `json:"stoppedAt,omitempty"`
	ImdbID          string  `json:"imdbId,omitempty"`
	TmdbID          int     `json:"tmdbId,omitempty"`
	TvdbID          int     `json:"tvdbId,omitempty"`
	RatingKey       string  `json:"ratingKey,omitempty"`
}

// TranscodeShare summarises how much of the user's own playback the server
// had to transcode instead of streaming directly.
type TranscodeShare struct {
	Plays      int `json:"plays"`
	Transcodes int `json:"transcodes"`
	// Devices breaks the Transcodes count down by the device that caused
	// it, most transcodes first. It answers the question the bare
	// percentage raises — which client is making the server work — and
	// therefore counts only transcoded plays, not all of them.
	Devices []DeviceTranscodes `json:"devices,omitempty"`
}

// DeviceTranscodes is one device's share of the transcoded plays.
type DeviceTranscodes struct {
	Device     string `json:"device"`
	Transcodes int    `json:"transcodes"`
}

// PopularTitle is one entry of the household's most-watched ranking.
type PopularTitle struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
	Plays     int    `json:"plays"`
	PosterURL string `json:"posterUrl,omitempty"`
	// Watched is the requesting user's own state, not the household's: the
	// ranking says what everyone here watches, the tick says whether you
	// have seen it yourself.
	Watched bool `json:"watched"`
	TmdbID  int  `json:"tmdbId,omitempty"`
}

// Watcher is one other household member who watched the same title.
type Watcher struct {
	Name                    string  `json:"name"`
	Plays                   int     `json:"plays"`
	CompletionPercent       float64 `json:"completionPercent"`
	LastWatchedDay          string  `json:"lastWatchedDay,omitempty"`
	DistinctEpisodesWatched *int    `json:"distinctEpisodesWatched,omitempty"`
}

// MediaStats is how often a title was played inside the household, as
// opposed to OMDb's public ratings.
type MediaStats struct {
	Plays       int   `json:"plays"`
	UniqueUsers int   `json:"uniqueUsers"`
	WatchTimeMS int64 `json:"watchTimeMs"`
}

// Reader is the surface the HTTP handlers depend on, so they can be tested
// against a fake instead of a live Tracearr.
type Reader interface {
	IdentityID(ctx context.Context, embyUserID string) (string, error)
	TopGenres(ctx context.Context, identityID string) ([]Genre, error)
	Unfinished(ctx context.Context, identityID string, since time.Time) ([]UnfinishedPlay, error)
	TranscodeShare(ctx context.Context, identityID string, since time.Time) (TranscodeShare, error)
	Watchers(ctx context.Context, ref string) ([]Watcher, error)
	MediaStats(ctx context.Context, ref string) (MediaStats, error)
	MostWatched(ctx context.Context, viewerID string, since time.Time, limit int) (movies, shows []PopularTitle, err error)
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient returns nil when either half of the configuration is missing,
// which is how every optional integration in this codebase spells
// "not configured" — handlers then need no enabled-check of their own.
func NewClient(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, httpClient: &http.Client{Timeout: 8 * time.Second}}
}

// Ref builds the type-qualified media reference the /media endpoints
// expect, e.g. "movie:tmdb:584". mediaType is Tracearr's own vocabulary
// ("movie", "show", "episode") — see EmbyMediaType for the mapping from
// Emby's item types. It returns "" when the caller has no usable id, which
// the Ref-taking methods treat as "nothing to look up".
func Ref(mediaType, provider, id string) string {
	mediaType, provider, id = strings.TrimSpace(mediaType), strings.TrimSpace(provider), strings.TrimSpace(id)
	if mediaType == "" || provider == "" || id == "" {
		return ""
	}
	return mediaType + ":" + provider + ":" + id
}

// EmbyMediaType maps an Emby item type onto Tracearr's media vocabulary.
// It returns "" for types Tracearr's provider refs cannot address (seasons
// carry no provider id of their own, and everything else is out of scope).
func EmbyMediaType(embyType string) string {
	switch strings.ToLower(strings.TrimSpace(embyType)) {
	case "movie":
		return "movie"
	case "series":
		return "show"
	case "episode":
		return "episode"
	default:
		return ""
	}
}

// IdentityID resolves an Emby user id to the Tracearr identity that owns
// it. Tracearr stores the media server's own user id verbatim on each
// linked account, so no mapping table is needed on this side.
func (client *Client) IdentityID(ctx context.Context, embyUserID string) (string, error) {
	if client == nil || strings.TrimSpace(embyUserID) == "" {
		return "", nil
	}

	cursor := ""
	for page := 0; page < maxPages; page++ {
		query := url.Values{"pageSize": {strconv.Itoa(pageSize)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		var response struct {
			Data []struct {
				ID       string `json:"id"`
				Accounts []struct {
					ServerType     string `json:"server_type"`
					ExternalUserID string `json:"external_user_id"`
				} `json:"accounts"`
			} `json:"data"`
			Meta struct {
				NextCursor *string `json:"nextCursor"`
			} `json:"meta"`
		}
		if !client.get(ctx, "/users", query, &response) {
			return "", nil
		}

		for _, identity := range response.Data {
			for _, account := range identity.Accounts {
				if account.ServerType == "emby" && strings.EqualFold(account.ExternalUserID, embyUserID) {
					return identity.ID, nil
				}
			}
		}
		if response.Meta.NextCursor == nil || *response.Meta.NextCursor == "" {
			break
		}
		cursor = *response.Meta.NextCursor
	}
	return "", nil
}

func (client *Client) TopGenres(ctx context.Context, identityID string) ([]Genre, error) {
	if client == nil || strings.TrimSpace(identityID) == "" {
		return nil, nil
	}

	var response struct {
		TopGenres []Genre `json:"top_genres"`
	}
	if !client.get(ctx, "/users/"+url.PathEscape(identityID)+"/stats", nil, &response) {
		return nil, nil
	}
	return response.TopGenres, nil
}

// Unfinished lists titles the user started but never finished, newest
// first and one entry per title. Plays that barely started or almost
// finished are dropped: below minProgress the user likely just sampled
// something, and above maxProgress Tracearr's own watched threshold is
// close enough that showing it as "unfinished" would read as wrong.
func (client *Client) Unfinished(ctx context.Context, identityID string, since time.Time) ([]UnfinishedPlay, error) {
	const (
		minProgress = 5.0
		maxProgress = 90.0
	)

	records, ok := client.history(ctx, identityID, since, map[string]string{"watched": "false"})
	if !ok {
		return nil, nil
	}

	seen := make(map[string]bool, len(records))
	var unfinished []UnfinishedPlay
	for _, record := range records {
		if record.PercentComplete < minProgress || record.PercentComplete > maxProgress {
			continue
		}
		// media_id is empty for items Tracearr could not identify; those
		// cannot be deduplicated or linked anywhere, so they are skipped.
		if record.MediaID == "" || seen[record.MediaID] {
			continue
		}
		seen[record.MediaID] = true
		unfinished = append(unfinished, UnfinishedPlay{
			MediaID:         record.MediaID,
			MediaType:       record.MediaType,
			Title:           record.MediaTitle,
			ShowTitle:       record.ShowTitle,
			SeasonNumber:    record.SeasonNumber,
			EpisodeNumber:   record.EpisodeNumber,
			Year:            record.Year,
			PercentComplete: record.PercentComplete,
			StoppedAt:       record.StoppedAt,
			ImdbID:          record.ImdbID,
			TmdbID:          record.TmdbID,
			TvdbID:          record.TvdbID,
			RatingKey:       record.RatingKey,
		})
	}
	return unfinished, nil
}

func (client *Client) TranscodeShare(ctx context.Context, identityID string, since time.Time) (TranscodeShare, error) {
	records, ok := client.history(ctx, identityID, since, nil)
	if !ok {
		return TranscodeShare{}, nil
	}

	var share TranscodeShare
	perDevice := map[string]int{}
	for _, record := range records {
		share.Plays++
		if !record.IsTranscode {
			continue
		}
		share.Transcodes++
		// Tracearr leaves device null for playback it could not attribute
		// to a client. Those still count towards the total — dropping them
		// would make the device list disagree with the percentage above it.
		if device := strings.TrimSpace(record.Device); device != "" {
			perDevice[device]++
		}
	}

	share.Devices = make([]DeviceTranscodes, 0, len(perDevice))
	for device, transcodes := range perDevice {
		share.Devices = append(share.Devices, DeviceTranscodes{Device: device, Transcodes: transcodes})
	}
	// Map iteration is random, so an equal-count tie would otherwise make the
	// list reshuffle on every poll. Name is the tie-breaker for a stable order.
	sort.Slice(share.Devices, func(i, j int) bool {
		if share.Devices[i].Transcodes != share.Devices[j].Transcodes {
			return share.Devices[i].Transcodes > share.Devices[j].Transcodes
		}
		return share.Devices[i].Device < share.Devices[j].Device
	})
	return share, nil
}

// MostWatched ranks what the whole household played since `since`, split
// into movies and shows, most plays first. Tracearr has no endpoint for
// this — its own dashboard computes it internally — so the ranking is
// counted here from the raw history feed.
//
// Episodes are folded into their show: a series is popular because people
// watched twelve of its episodes, and listing those twelve separately would
// crowd out everything else.
func (client *Client) MostWatched(ctx context.Context, viewerID string, since time.Time, limit int) ([]PopularTitle, []PopularTitle, error) {
	records, ok := client.householdHistory(ctx, since)
	if !ok {
		return nil, nil, nil
	}

	movies := map[string]*PopularTitle{}
	shows := map[string]*PopularTitle{}
	for _, record := range records {
		var (
			group map[string]*PopularTitle
			key   string
			title string
		)
		switch record.MediaType {
		case "movie":
			group, key, title = movies, record.MediaID, record.MediaTitle
		case "episode":
			// show_media_id is the canonical parent id; without it the
			// episode cannot be attributed to a series at all.
			group, key, title = shows, record.ShowMediaID, record.ShowTitle
		default:
			// Music and anything else Tracearr tracks has no place in a
			// movies/shows ranking.
			continue
		}
		if key == "" || title == "" {
			continue
		}

		entry := group[key]
		if entry == nil {
			entry = &PopularTitle{ID: key, Title: title, Year: record.Year, PosterURL: record.PosterURL, TmdbID: record.TmdbID}
			group[key] = entry
		}
		entry.Plays++
		// The first record of a title may be one whose poster never
		// resolved; a later one can still supply it.
		if entry.PosterURL == "" {
			entry.PosterURL = record.PosterURL
		}
		if record.Watched && record.User.ID != "" && record.User.ID == viewerID {
			entry.Watched = true
		}
	}
	return rankTitles(movies, limit), rankTitles(shows, limit), nil
}

func rankTitles(grouped map[string]*PopularTitle, limit int) []PopularTitle {
	ranked := make([]PopularTitle, 0, len(grouped))
	for _, entry := range grouped {
		ranked = append(ranked, *entry)
	}
	// Map iteration is random, so without the title tie-breaker two titles
	// on the same play count would swap places on every poll — and these
	// rows carry visible rank numbers.
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Plays != ranked[j].Plays {
			return ranked[i].Plays > ranked[j].Plays
		}
		return ranked[i].Title < ranked[j].Title
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func (client *Client) Watchers(ctx context.Context, ref string) ([]Watcher, error) {
	if client == nil || strings.TrimSpace(ref) == "" {
		return nil, nil
	}

	var response struct {
		Watchers []struct {
			User struct {
				Username     string `json:"username"`
				IdentityName string `json:"identity_name"`
			} `json:"user"`
			Plays                   int     `json:"plays"`
			CompletionPct           float64 `json:"completion_pct"`
			LastWatchedDay          string  `json:"last_watched_day"`
			DistinctEpisodesWatched *int    `json:"distinct_episodes_watched"`
		} `json:"watchers"`
	}
	if !client.get(ctx, "/media/"+url.PathEscape(ref)+"/watchers", nil, &response) {
		return nil, nil
	}

	watchers := make([]Watcher, 0, len(response.Watchers))
	for _, entry := range response.Watchers {
		name := entry.User.IdentityName
		if name == "" {
			name = entry.User.Username
		}
		watchers = append(watchers, Watcher{
			Name:                    name,
			Plays:                   entry.Plays,
			CompletionPercent:       entry.CompletionPct,
			LastWatchedDay:          entry.LastWatchedDay,
			DistinctEpisodesWatched: entry.DistinctEpisodesWatched,
		})
	}
	return watchers, nil
}

// MediaStats reports all-time household plays for one title. per_server is
// ignored on purpose: this install tracks a single Emby server, and the
// combined figures already answer "how popular is this here".
func (client *Client) MediaStats(ctx context.Context, ref string) (MediaStats, error) {
	if client == nil || strings.TrimSpace(ref) == "" {
		return MediaStats{}, nil
	}

	// MediaStats carries this API's own camelCase tags for the browser, so
	// Tracearr's snake_case payload needs its own decode target.
	var response struct {
		Windows struct {
			AllTime struct {
				Combined struct {
					Plays       int   `json:"plays"`
					WatchTimeMS int64 `json:"watch_time_ms"`
					UniqueUsers int   `json:"unique_users"`
				} `json:"combined"`
			} `json:"all_time"`
		} `json:"windows"`
	}
	if !client.get(ctx, "/media/"+url.PathEscape(ref)+"/stats", nil, &response) {
		return MediaStats{}, nil
	}
	combined := response.Windows.AllTime.Combined
	return MediaStats{Plays: combined.Plays, UniqueUsers: combined.UniqueUsers, WatchTimeMS: combined.WatchTimeMS}, nil
}

// historyRecord is the subset of Tracearr's HistoryRecord this package uses.
type historyRecord struct {
	MediaID         string  `json:"media_id"`
	MediaType       string  `json:"media_type"`
	MediaTitle      string  `json:"media_title"`
	ShowTitle       string  `json:"show_title"`
	SeasonNumber    int     `json:"season_number"`
	EpisodeNumber   int     `json:"episode_number"`
	Year            int     `json:"year"`
	PercentComplete float64 `json:"percent_complete"`
	StoppedAt       string  `json:"stopped_at"`
	IsTranscode     bool    `json:"is_transcode"`
	Device          string  `json:"device"`
	Watched         bool    `json:"watched"`
	PosterURL       string  `json:"poster_url"`
	ShowMediaID     string  `json:"show_media_id"`
	User            struct {
		ID string `json:"id"`
	} `json:"user"`
	ImdbID    string `json:"imdb_id"`
	TmdbID    int    `json:"tmdb_id"`
	TvdbID    int    `json:"tvdb_id"`
	RatingKey string `json:"rating_key"`
}

// history walks the global /history feed filtered to one identity. The
// per-identity /users/{id}/history route takes no since or watched filter,
// so the global route with user_id is the one that can push both filters
// upstream instead of over-fetching and discarding here.
func (client *Client) history(ctx context.Context, identityID string, since time.Time, extra map[string]string) ([]historyRecord, bool) {
	if client == nil || strings.TrimSpace(identityID) == "" {
		return nil, false
	}
	query := url.Values{"user_id": {identityID}}
	for key, value := range extra {
		query.Set(key, value)
	}
	return client.historyPages(ctx, query, since, maxPages)
}

// householdHistory is history without the user_id filter: every account's
// plays, which is what a "most watched here" list has to count. It is the
// only caller that deliberately reads other people's viewing, so it stays a
// separate, named entry point rather than an optional argument on history.
func (client *Client) householdHistory(ctx context.Context, since time.Time) ([]historyRecord, bool) {
	if client == nil {
		return nil, false
	}
	return client.historyPages(ctx, url.Values{}, since, maxHouseholdPages)
}

// historyPages walks the cursor-paginated /history feed. query carries the
// caller's filters; pageSize, since and the cursor are added here.
func (client *Client) historyPages(ctx context.Context, query url.Values, since time.Time, pageLimit int) ([]historyRecord, bool) {
	var (
		records []historyRecord
		cursor  string
	)
	for page := 0; page < pageLimit; page++ {
		pageQuery := url.Values{"pageSize": {strconv.Itoa(pageSize)}}
		for key, values := range query {
			pageQuery[key] = values
		}
		if !since.IsZero() {
			pageQuery.Set("since", since.UTC().Format(time.RFC3339))
		}
		if cursor != "" {
			pageQuery.Set("cursor", cursor)
		}

		var response struct {
			Data []historyRecord `json:"data"`
			Meta struct {
				NextCursor *string `json:"nextCursor"`
			} `json:"meta"`
		}
		if !client.get(ctx, "/history", pageQuery, &response) {
			return nil, false
		}

		records = append(records, response.Data...)
		if response.Meta.NextCursor == nil || *response.Meta.NextCursor == "" {
			break
		}
		cursor = *response.Meta.NextCursor
	}
	return records, true
}

// ImageURL resolves a poster reference from a history record against the
// configured instance, and refuses anything that does not belong to it.
//
// Tracearr hands back relative paths like
// "/api/v1/images/proxy?server=…&url=…". Those are attacker-influenced only
// as far as Tracearr itself is concerned, but the value still reaches us
// over the wire and is passed back in by the browser, so the host is pinned
// to the configured base rather than trusted.
func (client *Client) ImageURL(raw string) (string, bool) {
	if client == nil {
		return "", false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	base, err := url.Parse(client.baseURL)
	if err != nil {
		return "", false
	}
	target, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	// A relative reference inherits the base entirely; an absolute one must
	// match it, scheme, host and port alike. ResolveReference handles the
	// first case and leaves an absolute URL untouched, so both end up
	// checked by the same comparison.
	resolved := base.ResolveReference(target)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return "", false
	}
	return resolved.String(), true
}

// FetchImage loads one image from the Tracearr instance. Tracearr's image
// proxy is deliberately unauthenticated (its own note: so plain <img> tags
// work), so no key is sent — but the request still goes through this
// package because the instance usually lives on a private address that the
// server's general-purpose image fetcher blocks on purpose.
func (client *Client) FetchImage(ctx context.Context, rawURL string) (io.ReadCloser, bool) {
	endpoint, ok := client.ImageURL(rawURL)
	if !ok {
		return nil, false
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, false
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, false
	}
	return response.Body, true
}

// get performs one authenticated GET and decodes it into target. It reports
// success as a bool rather than an error because every caller treats a
// failure the same way — return the zero value and let the page render
// without the Tracearr extras. Nothing here is worth failing a request over.
func (client *Client) get(ctx context.Context, path string, query url.Values, target any) bool {
	endpoint := client.baseURL + "/api/v2/public" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false
	}
	return json.NewDecoder(response.Body).Decode(target) == nil
}

var _ Reader = (*Client)(nil)
