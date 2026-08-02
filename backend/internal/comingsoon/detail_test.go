package comingsoon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// calendarServer answers both calendars with the full resource Radarr and
// Sonarr really return, so the detail fields have somewhere to come from.
func calendarServer(t *testing.T) *httptest.Server {
	t.Helper()
	now := time.Now().UTC()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/radarr/"):
			fmt.Fprintf(writer, `[{"title":"Cinema Film","tmdbId":7,"inCinemas":%q,"digitalRelease":%q,
				"overview":"Ein Film.","runtime":123,"year":2026,"certification":"FSK 12","studio":"Studio X",
				"status":"released","genres":["Action","Drama"],"ratings":{"tmdb":{"value":7.5}},
				"images":[{"coverType":"poster","remoteUrl":"https://image.tmdb.org/t/p/w500/movie.jpg"},
				          {"coverType":"fanart","remoteUrl":"https://image.tmdb.org/t/p/w1280/back.jpg"}]}]`,
				now.AddDate(0, 0, -2).Format(time.RFC3339), now.AddDate(0, 0, 5).Format(time.RFC3339))
		case strings.Contains(request.URL.Path, "/sonarr/"):
			fmt.Fprintf(writer, `[{"title":"Neue Folge","seasonNumber":2,"episodeNumber":3,"airDateUtc":%q,
				"series":{"title":"Eine Serie","tvdbId":42,"overview":"Eine Serie.","year":2024,"runtime":45,
				"network":"ARD","status":"continuing","certification":"FSK 16","genres":["Krimi"],
				"ratings":{"value":8.1},
				"images":[{"coverType":"poster","remoteUrl":"https://artworks.thetvdb.com/banners/show.jpg"}]}}]`,
				now.AddDate(0, 0, 2).Format(time.RFC3339))
		default:
			http.NotFound(writer, request)
		}
	}))
}

// TestSeriesKeepAnIdWithoutTMDB is the regression for the bug this feature
// uncovered: translating Sonarr's TVDB id into a TMDB one is itself a TMDB
// call, so without TMDB configured every series entry was appended with an
// empty id — a tile that rendered but could never open.
func TestSeriesKeepAnIdWithoutTMDB(t *testing.T) {
	server := calendarServer(t)
	defer server.Close()
	client := NewClient(server.URL+"/radarr", "k", server.URL+"/sonarr", "k", "", "DE", 28)

	items, err := client.Upcoming(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var series *Item
	for index := range items {
		if items[index].MediaType == "tv" {
			series = &items[index]
		}
	}
	if series == nil {
		t.Fatal("no series entry in the upcoming list")
	}
	if series.ID == "" {
		t.Error("series entry has an empty ID — its detail screen can never open")
	}
	if series.Source != SourceSonarr || series.DetailID != "42" {
		t.Errorf("Source/DetailID = %q/%q, want sonarr/42", series.Source, series.DetailID)
	}
	if series.TmdbID != "" {
		t.Errorf("TmdbID = %q, want empty without TMDB configured", series.TmdbID)
	}
}

func TestDetailFieldsComeFromTheCalendar(t *testing.T) {
	server := calendarServer(t)
	defer server.Close()
	client := NewClient(server.URL+"/radarr", "k", server.URL+"/sonarr", "k", "", "DE", 28)

	movie, found, err := client.Detail(context.Background(), SourceRadarr, "7")
	if err != nil || !found {
		t.Fatalf("Detail(radarr, 7) found=%v err=%v", found, err)
	}
	if movie.Overview != "Ein Film." || movie.RuntimeMinutes != 123 || movie.Year != 2026 {
		t.Errorf("movie detail = %+v", movie)
	}
	if movie.OfficialRating != "FSK 12" || movie.Studio != "Studio X" || movie.Rating != 7.5 {
		t.Errorf("movie certification/studio/rating = %q/%q/%v", movie.OfficialRating, movie.Studio, movie.Rating)
	}
	if len(movie.Genres) != 2 {
		t.Errorf("genres = %v", movie.Genres)
	}
	if !strings.HasPrefix(movie.BackdropURL, "/api/artwork?") {
		t.Errorf("BackdropURL = %q, want it proxied", movie.BackdropURL)
	}

	series, found, err := client.Detail(context.Background(), SourceSonarr, "42")
	if err != nil || !found {
		t.Fatalf("Detail(sonarr, 42) found=%v err=%v", found, err)
	}
	if series.Overview != "Eine Serie." || series.Studio != "ARD" || series.RuntimeMinutes != 45 {
		t.Errorf("series detail = %+v", series)
	}
}

func TestDetailRejectsUnknownIdsAndSources(t *testing.T) {
	server := calendarServer(t)
	defer server.Close()
	client := NewClient(server.URL+"/radarr", "k", server.URL+"/sonarr", "k", "", "DE", 28)

	for _, testCase := range []struct{ source, id string }{
		{SourceRadarr, "999"},
		{SourceSonarr, "999"},
		{SourceRadarr, "42"}, // Sonarr's id must not resolve through Radarr
		{"", "7"},
		{SourceRadarr, ""},
	} {
		if _, found, err := client.Detail(context.Background(), testCase.source, testCase.id); found || err != nil {
			t.Errorf("Detail(%q, %q) found=%v err=%v, want not found", testCase.source, testCase.id, found, err)
		}
	}
}
