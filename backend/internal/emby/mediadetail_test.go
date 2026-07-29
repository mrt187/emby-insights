package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbyMediaDetailSplitsCastAndCrew(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items/154950" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"Id":"154950","Name":"Dune","Type":"Movie","Overview":"Sci-fi epic",
			"Genres":["Science Fiction"],"CommunityRating":7.8,"OfficialRating":"12",
			"ProductionYear":2026,"RunTimeTicks":79511420000,
			"ImageTags":{"Primary":"poster-tag"},"BackdropImageTags":["backdrop-tag"],
			"People":[
				{"Id":"1","Name":"Denis Villeneuve","Type":"Director","PrimaryImageTag":"tag-1"},
				{"Id":"2","Name":"Timothée Chalamet","Role":"Paul Atreides","Type":"Actor","PrimaryImageTag":"tag-2"}
			],
			"UserData":{"Played":true,"UnplayedItemCount":0}
		}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "154950")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if detail.Title != "Dune" || detail.Overview != "Sci-fi epic" || !detail.Played {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.RuntimeMinutes != 132 {
		t.Fatalf("RuntimeMinutes = %d, want 132", detail.RuntimeMinutes)
	}
	if detail.PosterURL == "" || detail.BackdropURL == "" {
		t.Fatalf("expected poster and backdrop URLs, got %#v", detail)
	}
	if len(detail.Cast) != 1 || detail.Cast[0].Name != "Timothée Chalamet" || detail.Cast[0].Role != "Paul Atreides" {
		t.Fatalf("cast = %#v", detail.Cast)
	}
	if len(detail.Crew) != 1 || detail.Crew[0].Name != "Denis Villeneuve" || detail.Crew[0].Role != "Director" {
		t.Fatalf("crew = %#v", detail.Crew)
	}
}

func TestEmbyMediaDetailComputesSeriesProgressAndSeasons(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{
				"Id":"1","Name":"Severance","Type":"Series",
				"RecursiveItemCount":18,
				"UserData":{"Played":false,"UnplayedItemCount":3}
			}`))
		case request.URL.Path == "/emby/Shows/1/Seasons":
			if request.URL.Query().Get("UserId") != "user-1" {
				t.Fatalf("UserId = %q", request.URL.Query().Get("UserId"))
			}
			_, _ = writer.Write([]byte(`{"Items":[
				{"Id":"10","Name":"Staffel 1","IndexNumber":1,"RecursiveItemCount":9,"ImageTags":{"Primary":"tag-s1"},"UserData":{"Played":true,"UnplayedItemCount":0}},
				{"Id":"11","Name":"Staffel 2","IndexNumber":2,"RecursiveItemCount":9,"UserData":{"Played":false,"UnplayedItemCount":3}},
				{"Id":"12","Name":"Specials","IndexNumber":0,"RecursiveItemCount":0,"UserData":{"Played":false,"UnplayedItemCount":0}}
			]}`))
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "1")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if !detail.IsSeries || detail.TotalEpisodes != 18 || detail.WatchedEpisodes != 15 {
		t.Fatalf("detail = %#v", detail)
	}
	// Regression: a series with no crew entries (common — Emby often lists
	// only actors for TV shows) must serialize Crew as `[]`, not `null`, or
	// the frontend's array spread crashes.
	if detail.Crew == nil || detail.Cast == nil || detail.Genres == nil {
		t.Fatalf("Crew/Cast/Genres must be non-nil empty slices, got %#v", detail)
	}
	if len(detail.Seasons) != 2 {
		t.Fatalf("Seasons = %#v, want 2 (the empty 'Specials' season is skipped)", detail.Seasons)
	}
	if detail.Seasons[0].Title != "Staffel 1" || detail.Seasons[0].WatchedEpisodes != 9 || detail.Seasons[0].TotalEpisodes != 9 || !detail.Seasons[0].Played {
		t.Fatalf("Seasons[0] = %#v", detail.Seasons[0])
	}
	if detail.Seasons[0].PosterURL == "" {
		t.Fatalf("expected a poster URL for season 1")
	}
	if detail.Seasons[1].Title != "Staffel 2" || detail.Seasons[1].WatchedEpisodes != 6 || detail.Seasons[1].TotalEpisodes != 9 || detail.Seasons[1].Played {
		t.Fatalf("Seasons[1] = %#v", detail.Seasons[1])
	}
}

func TestEmbyMediaDetailResolvesEpisodeToItsSeries(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/emby/Users/user-1/Items/ep-9":
			_, _ = writer.Write([]byte(`{
				"Id":"ep-9","Name":"Semper I","Type":"Episode",
				"SeriesId":"1","ParentIndexNumber":2,"IndexNumber":5
			}`))
		case request.URL.Path == "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{
				"Id":"1","Name":"Severance","Type":"Series",
				"RecursiveItemCount":18,
				"UserData":{"Played":false,"UnplayedItemCount":3}
			}`))
		case request.URL.Path == "/emby/Shows/1/Seasons":
			_, _ = writer.Write([]byte(`{"Items":[]}`))
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "ep-9")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if detail.Title != "Severance" || !detail.IsSeries {
		t.Fatalf("detail = %#v, want the series resolved instead of the episode", detail)
	}
	if detail.CurrentSeasonNumber != 2 || detail.CurrentEpisodeNumber != 5 {
		t.Fatalf("CurrentSeasonNumber/CurrentEpisodeNumber = %d/%d, want 2/5", detail.CurrentSeasonNumber, detail.CurrentEpisodeNumber)
	}
}

func TestEmbyMediaDetailMovieHasNoSeasons(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items/154950" {
			t.Fatalf("unexpected request %q", request.URL.String())
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Id":"154950","Name":"Dune","Type":"Movie"}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "154950")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if detail.Seasons == nil || len(detail.Seasons) != 0 {
		t.Fatalf("Seasons = %#v, want empty slice", detail.Seasons)
	}
}
