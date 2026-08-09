package tracearr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := &Client{baseURL: server.URL, apiKey: "test-key", httpClient: server.Client()}
	return client, server
}

func TestNewClientRequiresBothHalves(t *testing.T) {
	if NewClient("", "key") != nil {
		t.Fatal("empty base URL should yield a nil client")
	}
	if NewClient("http://tracearr", "") != nil {
		t.Fatal("empty API key should yield a nil client")
	}
	client := NewClient("http://tracearr:3333/", "key")
	if client == nil {
		t.Fatal("fully configured NewClient returned nil")
	}
	if client.baseURL != "http://tracearr:3333" {
		t.Fatalf("baseURL = %q, want the trailing slash trimmed", client.baseURL)
	}
}

func TestIdentityIDMatchesEmbyAccountAcrossPages(t *testing.T) {
	var seenCursors []string
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if request.URL.Path != "/api/v2/public/users" {
			t.Errorf("path = %q", request.URL.Path)
		}
		seenCursors = append(seenCursors, request.URL.Query().Get("cursor"))
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("cursor") == "" {
			// The first page holds only a Jellyfin account carrying the same
			// external id, which must not match an Emby lookup.
			_, _ = writer.Write([]byte(`{"data":[
				{"id":"identity-1","accounts":[{"server_type":"jellyfin","external_user_id":"emby-guid"}]}
			],"meta":{"nextCursor":"page-2","pageSize":100}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"data":[
			{"id":"identity-2","accounts":[{"server_type":"emby","external_user_id":"emby-guid"}]}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	identityID, err := client.IdentityID(context.Background(), "emby-guid")
	if err != nil {
		t.Fatalf("IdentityID() error = %v", err)
	}
	if identityID != "identity-2" {
		t.Fatalf("identityID = %q, want identity-2", identityID)
	}
	if len(seenCursors) != 2 || seenCursors[1] != "page-2" {
		t.Fatalf("cursors = %v, want the second page to be fetched", seenCursors)
	}
}

func TestIdentityIDReturnsEmptyWhenUnknown(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	identityID, err := client.IdentityID(context.Background(), "emby-guid")
	if err != nil || identityID != "" {
		t.Fatalf("identityID = %q, err = %v", identityID, err)
	}
}

func TestTopGenres(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/public/users/identity-1/stats" {
			t.Errorf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"user_id":"identity-1","top_genres":[
			{"genre":"Drama","plays":42},{"genre":"Comedy","plays":17}
		]}`))
	})
	defer server.Close()

	genres, err := client.TopGenres(context.Background(), "identity-1")
	if err != nil {
		t.Fatalf("TopGenres() error = %v", err)
	}
	if len(genres) != 2 || genres[0].Genre != "Drama" || genres[0].Plays != 42 {
		t.Fatalf("genres = %#v", genres)
	}
}

func TestUnfinishedFiltersAndDeduplicates(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/public/history" {
			t.Errorf("path = %q", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("user_id") != "identity-1" {
			t.Errorf("user_id = %q", query.Get("user_id"))
		}
		if query.Get("watched") != "false" {
			t.Errorf("watched = %q, want the filter pushed upstream", query.Get("watched"))
		}
		if query.Get("since") == "" {
			t.Error("since was not sent")
		}
		_, _ = writer.Write([]byte(`{"data":[
			{"media_id":"m1","media_type":"movie","media_title":"Half Watched","percent_complete":48.5,"stopped_at":"2026-08-01T20:00:00Z","imdb_id":"tt1"},
			{"media_id":"m1","media_type":"movie","media_title":"Half Watched","percent_complete":30.0},
			{"media_id":"m2","media_type":"movie","media_title":"Barely Started","percent_complete":2.0},
			{"media_id":"m3","media_type":"movie","media_title":"Almost Done","percent_complete":95.0},
			{"media_id":"","media_type":"movie","media_title":"Unidentified","percent_complete":50.0}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	unfinished, err := client.Unfinished(context.Background(), "identity-1", time.Now().AddDate(0, -6, 0))
	if err != nil {
		t.Fatalf("Unfinished() error = %v", err)
	}
	if len(unfinished) != 1 {
		t.Fatalf("unfinished = %#v, want only the mid-progress play", unfinished)
	}
	if unfinished[0].MediaID != "m1" || unfinished[0].ImdbID != "tt1" || unfinished[0].PercentComplete != 48.5 {
		t.Fatalf("unfinished[0] = %#v", unfinished[0])
	}
}

func TestTranscodeShare(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("watched") != "" {
			t.Error("transcode share must not filter by watched state")
		}
		_, _ = writer.Write([]byte(`{"data":[
			{"media_id":"m1","is_transcode":true},
			{"media_id":"m2","is_transcode":false},
			{"media_id":"m3","is_transcode":true}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	share, err := client.TranscodeShare(context.Background(), "identity-1", time.Time{})
	if err != nil {
		t.Fatalf("TranscodeShare() error = %v", err)
	}
	if share.Plays != 3 || share.Transcodes != 2 {
		t.Fatalf("share = %#v, want 2 of 3", share)
	}
}

// The device list explains the percentage, so it counts transcoded plays
// only, orders them worst-first, and keeps a play whose device Tracearr
// could not attribute in the total rather than in the breakdown.
func TestTranscodeShareBreaksDownByDevice(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[
			{"media_id":"m1","is_transcode":true,"device":"iPhone"},
			{"media_id":"m2","is_transcode":true,"device":"Apple TV"},
			{"media_id":"m3","is_transcode":false,"device":"Apple TV"},
			{"media_id":"m4","is_transcode":true,"device":"Apple TV"},
			{"media_id":"m5","is_transcode":true,"device":null}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	share, err := client.TranscodeShare(context.Background(), "identity-1", time.Time{})
	if err != nil {
		t.Fatalf("TranscodeShare() error = %v", err)
	}
	if share.Plays != 5 || share.Transcodes != 4 {
		t.Fatalf("share = %d of %d plays, want 4 of 5", share.Transcodes, share.Plays)
	}
	want := []DeviceTranscodes{{Device: "Apple TV", Transcodes: 2}, {Device: "iPhone", Transcodes: 1}}
	if !reflect.DeepEqual(share.Devices, want) {
		t.Errorf("devices = %#v, want %#v", share.Devices, want)
	}
}

// A device that only ever direct-played must not show up in a list of what
// caused transcodes.
func TestTranscodeShareOmitsDevicesWithoutTranscodes(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[
			{"media_id":"m1","is_transcode":false,"device":"Apple TV"}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	share, err := client.TranscodeShare(context.Background(), "identity-1", time.Time{})
	if err != nil {
		t.Fatalf("TranscodeShare() error = %v", err)
	}
	if len(share.Devices) != 0 {
		t.Errorf("devices = %#v, want none", share.Devices)
	}
}

func TestWatchersPrefersIdentityName(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/public/media/movie:tmdb:584/watchers" {
			t.Errorf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"media_id":"m1","media_type":"movie","window":"all_time","watchers":[
			{"user":{"username":"alex_emby","identity_name":"Alex"},"plays":3,"completion_pct":98.2,"last_watched_day":"2026-08-01"},
			{"user":{"username":"nono","identity_name":""},"plays":1,"completion_pct":12.0}
		]}`))
	})
	defer server.Close()

	watchers, err := client.Watchers(context.Background(), Ref("movie", "tmdb", "584"))
	if err != nil {
		t.Fatalf("Watchers() error = %v", err)
	}
	if len(watchers) != 2 {
		t.Fatalf("watchers = %#v", watchers)
	}
	if watchers[0].Name != "Alex" || watchers[0].CompletionPercent != 98.2 {
		t.Fatalf("watchers[0] = %#v", watchers[0])
	}
	if watchers[1].Name != "nono" {
		t.Fatalf("watchers[1].Name = %q, want the server username as fallback", watchers[1].Name)
	}
}

func TestMediaStatsReadsAllTimeCombined(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"media_id":"m1","media_type":"movie","windows":{
			"all_time":{"combined":{"plays":12,"watch_time_ms":7200000,"unique_users":4},"per_server":[]},
			"last_30":{"combined":{"plays":2,"watch_time_ms":100,"unique_users":1},"per_server":[]}
		}}`))
	})
	defer server.Close()

	stats, err := client.MediaStats(context.Background(), "movie:tmdb:584")
	if err != nil {
		t.Fatalf("MediaStats() error = %v", err)
	}
	if stats.Plays != 12 || stats.UniqueUsers != 4 || stats.WatchTimeMS != 7200000 {
		t.Fatalf("stats = %#v", stats)
	}
}

// An unreachable or unhappy Tracearr must never surface as an error: these
// features decorate pages that have to render without them.
func TestUpstreamFailuresDegradeToZeroValues(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"statusCode":401,"error":"Unauthorized"}`))
	})
	defer server.Close()

	ctx := context.Background()
	if id, err := client.IdentityID(ctx, "emby-guid"); err != nil || id != "" {
		t.Fatalf("IdentityID: id = %q, err = %v", id, err)
	}
	if genres, err := client.TopGenres(ctx, "identity-1"); err != nil || genres != nil {
		t.Fatalf("TopGenres: genres = %#v, err = %v", genres, err)
	}
	if plays, err := client.Unfinished(ctx, "identity-1", time.Time{}); err != nil || plays != nil {
		t.Fatalf("Unfinished: plays = %#v, err = %v", plays, err)
	}
	if share, err := client.TranscodeShare(ctx, "identity-1", time.Time{}); err != nil || !reflect.DeepEqual(share, TranscodeShare{}) {
		t.Fatalf("TranscodeShare: share = %#v, err = %v", share, err)
	}
	if watchers, err := client.Watchers(ctx, "movie:tmdb:584"); err != nil || watchers != nil {
		t.Fatalf("Watchers: watchers = %#v, err = %v", watchers, err)
	}
	if stats, err := client.MediaStats(ctx, "movie:tmdb:584"); err != nil || stats != (MediaStats{}) {
		t.Fatalf("MediaStats: stats = %#v, err = %v", stats, err)
	}
}

func TestNilClientAndEmptyArgumentsAreInert(t *testing.T) {
	var client *Client
	ctx := context.Background()

	if id, err := client.IdentityID(ctx, "emby-guid"); err != nil || id != "" {
		t.Fatalf("nil IdentityID: id = %q, err = %v", id, err)
	}
	if genres, err := client.TopGenres(ctx, "identity-1"); err != nil || genres != nil {
		t.Fatalf("nil TopGenres: genres = %#v, err = %v", genres, err)
	}
	if plays, err := client.Unfinished(ctx, "identity-1", time.Time{}); err != nil || plays != nil {
		t.Fatalf("nil Unfinished: plays = %#v, err = %v", plays, err)
	}
	if share, err := client.TranscodeShare(ctx, "identity-1", time.Time{}); err != nil || !reflect.DeepEqual(share, TranscodeShare{}) {
		t.Fatalf("nil TranscodeShare: share = %#v, err = %v", share, err)
	}
	if watchers, err := client.Watchers(ctx, "movie:tmdb:584"); err != nil || watchers != nil {
		t.Fatalf("nil Watchers: watchers = %#v, err = %v", watchers, err)
	}
	if stats, err := client.MediaStats(ctx, "movie:tmdb:584"); err != nil || stats != (MediaStats{}) {
		t.Fatalf("nil MediaStats: stats = %#v, err = %v", stats, err)
	}

	// A configured client with nothing to look up must not call out either.
	configured, server := newTestClient(func(http.ResponseWriter, *http.Request) {
		t.Error("empty arguments must not reach the network")
	})
	defer server.Close()
	if _, err := configured.IdentityID(ctx, ""); err != nil {
		t.Fatalf("empty emby user id: %v", err)
	}
	if _, err := configured.TopGenres(ctx, ""); err != nil {
		t.Fatalf("empty identity id: %v", err)
	}
	if _, err := configured.Watchers(ctx, ""); err != nil {
		t.Fatalf("empty ref: %v", err)
	}
}

func TestRefAndEmbyMediaType(t *testing.T) {
	if got := Ref("movie", "tmdb", "584"); got != "movie:tmdb:584" {
		t.Fatalf("Ref() = %q", got)
	}
	if got := Ref("movie", "imdb", ""); got != "" {
		t.Fatalf("Ref() with a missing id = %q, want empty", got)
	}
	for embyType, want := range map[string]string{
		"Movie": "movie", "Series": "show", "Episode": "episode", "Season": "", "MusicAlbum": "",
	} {
		if got := EmbyMediaType(embyType); got != want {
			t.Fatalf("EmbyMediaType(%q) = %q, want %q", embyType, got, want)
		}
	}
}

func TestHistoryPaginationIsBounded(t *testing.T) {
	requests := 0
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		// Always advertise another page — the client must stop anyway.
		_, _ = writer.Write([]byte(`{"data":[{"media_id":"m1","is_transcode":false}],"meta":{"nextCursor":"more","pageSize":100}}`))
	})
	defer server.Close()

	if _, err := client.TranscodeShare(context.Background(), "identity-1", time.Time{}); err != nil {
		t.Fatalf("TranscodeShare() error = %v", err)
	}
	if requests != maxPages {
		t.Fatalf("requests = %d, want the %d-page cap", requests, maxPages)
	}
}

// MostWatched is counted here rather than fetched, so the counting is what
// needs proving: episodes fold into their show, movies stay separate, and
// the order is stable.
func TestMostWatchedRanksHouseholdPlays(t *testing.T) {
	var sawUserFilter bool
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("user_id") != "" {
			sawUserFilter = true
		}
		_, _ = writer.Write([]byte(`{"data":[
			{"media_type":"movie","media_id":"m1","media_title":"Dune","year":2021,"poster_url":"/p/dune.jpg","tmdb_id":438631,"rating_key":"emby-42","watched":true,"user":{"id":"me"}},
			{"media_type":"movie","media_id":"m1","media_title":"Dune","year":2021,"poster_url":"/p/dune.jpg","user":{"id":"someone-else"}},
			{"media_type":"movie","media_id":"m2","media_title":"Arrival","year":2016,"poster_url":"/p/arrival.jpg","watched":true,"user":{"id":"someone-else"}},
			{"media_type":"episode","show_media_id":"s1","show_title":"Silo","media_title":"Freedom Day","poster_url":"/p/silo.jpg","rating_key":"emby-ep1","grandparent_rating_key":"emby-silo","user":{"id":"me"}},
			{"media_type":"episode","show_media_id":"s1","show_title":"Silo","media_title":"Machines","poster_url":"/p/silo.jpg","user":{"id":"me"}},
			{"media_type":"episode","show_media_id":"s2","show_title":"Severance","media_title":"Good News","poster_url":"/p/sev.jpg","user":{"id":"me"}},
			{"media_type":"track","media_id":"t1","media_title":"Song","user":{"id":"me"}}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	movies, shows, err := client.MostWatched(context.Background(), "me", time.Time{}, 10)
	if err != nil {
		t.Fatalf("MostWatched() error = %v", err)
	}
	if sawUserFilter {
		t.Error("the household ranking must not filter by user_id")
	}

	if len(movies) != 2 || movies[0].Title != "Dune" || movies[0].Plays != 2 {
		t.Fatalf("movies = %#v, want Dune with 2 plays first", movies)
	}
	if movies[0].EmbyItemID != "emby-42" {
		t.Errorf("movies[0] lost the Emby item id: %#v", movies[0])
	}
	// A series' own Emby id is the episode's grandparent, not its rating key.
	if shows[0].EmbyItemID != "emby-silo" {
		t.Errorf("shows[0] took the episode id instead of the series id: %#v", shows[0])
	}
	if movies[0].Year != 2021 || movies[0].PosterURL != "/p/dune.jpg" || movies[0].TmdbID != 438631 {
		t.Errorf("movies[0] lost its metadata: %#v", movies[0])
	}
	// Two episodes of one show are one popular show, not two entries.
	if len(shows) != 2 || shows[0].Title != "Silo" || shows[0].Plays != 2 || shows[0].ID != "s1" {
		t.Fatalf("shows = %#v, want Silo grouped to 2 plays", shows)
	}
	// The music track must not land in either ranking.
	for _, entry := range append(append([]PopularTitle{}, movies...), shows...) {
		if entry.Title == "Song" {
			t.Error("a music track leaked into the ranking")
		}
	}
}

// The tick is personal: a title everyone else finished is not one you have
// seen, and a title you started but abandoned is not watched either.
func TestMostWatchedMarksOnlyTheViewersOwnWatches(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[
			{"media_type":"movie","media_id":"m1","media_title":"Seen By Me","watched":true,"user":{"id":"me"}},
			{"media_type":"movie","media_id":"m2","media_title":"Seen By Others","watched":true,"user":{"id":"housemate"}},
			{"media_type":"movie","media_id":"m3","media_title":"Abandoned By Me","watched":false,"user":{"id":"me"}}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	movies, _, err := client.MostWatched(context.Background(), "me", time.Time{}, 10)
	if err != nil {
		t.Fatalf("MostWatched() error = %v", err)
	}
	want := map[string]bool{"Seen By Me": true, "Seen By Others": false, "Abandoned By Me": false}
	for _, entry := range movies {
		if entry.Watched != want[entry.Title] {
			t.Errorf("%q watched = %v, want %v", entry.Title, entry.Watched, want[entry.Title])
		}
	}
}

// Equal play counts must not reshuffle between polls — these rows show rank
// numbers, so a wobbling order is visible to the user.
func TestMostWatchedOrdersTiesByTitleAndHonoursTheLimit(t *testing.T) {
	client, server := newTestClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[
			{"media_type":"movie","media_id":"m1","media_title":"Charlie","user":{"id":"me"}},
			{"media_type":"movie","media_id":"m2","media_title":"Alpha","user":{"id":"me"}},
			{"media_type":"movie","media_id":"m3","media_title":"Bravo","user":{"id":"me"}}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	for attempt := 0; attempt < 5; attempt++ {
		movies, _, err := client.MostWatched(context.Background(), "me", time.Time{}, 2)
		if err != nil {
			t.Fatalf("MostWatched() error = %v", err)
		}
		if len(movies) != 2 || movies[0].Title != "Alpha" || movies[1].Title != "Bravo" {
			t.Fatalf("attempt %d: movies = %#v, want Alpha then Bravo, capped at 2", attempt, movies)
		}
	}
}

// A busy household spans many pages; stopping after the first would drop
// titles out of the ranking without any sign that it happened.
func TestMostWatchedFollowsTheCursor(t *testing.T) {
	var pages int
	client, server := newTestClient(func(writer http.ResponseWriter, request *http.Request) {
		pages++
		if request.URL.Query().Get("cursor") == "" {
			_, _ = writer.Write([]byte(`{"data":[
				{"media_type":"movie","media_id":"m1","media_title":"Dune","user":{"id":"me"}}
			],"meta":{"nextCursor":"page-2","pageSize":100}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"data":[
			{"media_type":"movie","media_id":"m1","media_title":"Dune","user":{"id":"me"}}
		],"meta":{"nextCursor":null,"pageSize":100}}`))
	})
	defer server.Close()

	movies, _, err := client.MostWatched(context.Background(), "me", time.Time{}, 10)
	if err != nil {
		t.Fatalf("MostWatched() error = %v", err)
	}
	if pages != 2 {
		t.Fatalf("fetched %d pages, want both", pages)
	}
	if len(movies) != 1 || movies[0].Plays != 2 {
		t.Fatalf("movies = %#v, want Dune counted across both pages", movies)
	}
}

func TestMostWatchedIsInertWithoutAClient(t *testing.T) {
	var client *Client
	movies, shows, err := client.MostWatched(context.Background(), "me", time.Time{}, 10)
	if err != nil || movies != nil || shows != nil {
		t.Fatalf("movies = %#v, shows = %#v, err = %v", movies, shows, err)
	}
}

// ImageURL is the guard that keeps the poster route from becoming a way to
// reach anything else on the network the server sits in — the general image
// fetcher's private-address block does not apply to this path.
func TestImageURLPinsToTheConfiguredInstance(t *testing.T) {
	client := NewClient("http://tracearr.local:3333", "key")

	resolved, ok := client.ImageURL("/api/v1/images/proxy?server=abc&url=%2Flibrary%2Fx.jpg")
	if !ok || resolved != "http://tracearr.local:3333/api/v1/images/proxy?server=abc&url=%2Flibrary%2Fx.jpg" {
		t.Fatalf("relative poster path resolved to %q (ok=%v)", resolved, ok)
	}
	if resolved, ok := client.ImageURL("http://tracearr.local:3333/api/v1/images/proxy?x=1"); !ok {
		t.Errorf("the instance's own absolute URL was rejected: %q", resolved)
	}

	for _, raw := range []string{
		"",
		"   ",
		"http://evil.example/x.jpg",
		"https://tracearr.local:3333/x.jpg", // scheme must match too
		"http://tracearr.local:9999/x.jpg",  // different port is a different service
		"http://tracearr.local.evil/x.jpg",  // suffix trick
		"http://user@evil.example/x.jpg",
		"file:///etc/passwd",
	} {
		if resolved, ok := client.ImageURL(raw); ok {
			t.Errorf("BYPASS: %q resolved to %q, want rejection", raw, resolved)
		}
	}

	var nilClient *Client
	if _, ok := nilClient.ImageURL("/x.jpg"); ok {
		t.Error("a nil client must resolve nothing")
	}
}
