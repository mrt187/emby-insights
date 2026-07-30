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

func TestClientBuildsUpcomingAndCinemaRowsWithoutEmby(t *testing.T) {
	now := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Api-Key") != "calendar-key" {
			t.Fatalf("X-Api-Key = %q", request.Header.Get("X-Api-Key"))
		}
		switch {
		case strings.Contains(request.URL.Path, "/radarr/"):
			fmt.Fprintf(writer, `[{"title":"Cinema Film","tmdbId":7,"inCinemas":%q,"digitalRelease":%q,"images":[{"coverType":"poster","remoteUrl":"https://image/movie.jpg"}]}]`, now.AddDate(0, 0, -2).Format(time.RFC3339), now.AddDate(0, 0, 5).Format(time.RFC3339))
		case strings.Contains(request.URL.Path, "/sonarr/"):
			fmt.Fprintf(writer, `[{"title":"Neue Folge","seasonNumber":2,"episodeNumber":3,"airDateUtc":%q,"series":{"title":"Eine Serie","tvdbId":42,"images":[{"coverType":"poster","remoteUrl":"https://image/show.jpg"}]}}]`, now.AddDate(0, 0, 2).Format(time.RFC3339))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/radarr", "calendar-key", server.URL+"/sonarr", "calendar-key", "", "DE", 28)
	upcoming, err := client.Upcoming(context.Background())
	if err != nil {
		t.Fatalf("Upcoming() error = %v", err)
	}
	if len(upcoming) != 2 || upcoming[0].Title != "Eine Serie" || upcoming[0].SeasonNumber != 2 || upcoming[1].Title != "Cinema Film" {
		t.Fatalf("Upcoming() = %#v", upcoming)
	}
	cinema, err := client.InCinemas(context.Background())
	if err != nil {
		t.Fatalf("InCinemas() error = %v", err)
	}
	if len(cinema) != 1 || cinema[0].Title != "Cinema Film" || cinema[0].CinemaEndDate == "" {
		t.Fatalf("InCinemas() = %#v", cinema)
	}
}

func TestNewClientDisablesCalendarWithoutSources(t *testing.T) {
	client := NewClient("", "", "", "", "", "DE", 28)
	if client != nil {
		t.Fatal("NewClient() = non-nil without a source")
	}
}

func TestInCinemasUsesThirtyDayWindow(t *testing.T) {
	now := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, `[
			{"title":"Recent","tmdbId":1,"inCinemas":%q,"digitalRelease":%q},
			{"title":"Too old","tmdbId":2,"inCinemas":%q,"digitalRelease":%q},
			{"title":"Soon","tmdbId":3,"inCinemas":%q,"digitalRelease":%q}
		]`,
			now.AddDate(0, 0, -29).Format(time.RFC3339), now.AddDate(0, 0, 10).Format(time.RFC3339),
			now.AddDate(0, 0, -31).Format(time.RFC3339), now.AddDate(0, 0, 10).Format(time.RFC3339),
			now.AddDate(0, 0, 29).Format(time.RFC3339), now.AddDate(0, 0, 40).Format(time.RFC3339),
		)
	}))
	defer server.Close()

	items, err := NewClient(server.URL, "calendar-key", "", "", "", "DE", 28).InCinemas(context.Background())
	if err != nil {
		t.Fatalf("InCinemas() error = %v", err)
	}
	if len(items) != 2 || items[0].Title != "Recent" || items[1].Title != "Soon" {
		t.Fatalf("InCinemas() = %#v", items)
	}
}
