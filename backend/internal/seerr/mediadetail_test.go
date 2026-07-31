package seerr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaDetailParsesMovie(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/movie/1228710" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"title":"The Mandalorian and Grogu","overview":"Bounty hunter saga",
			"posterPath":"/poster.jpg","backdropPath":"/backdrop.jpg",
			"voteAverage":7.3,"releaseDate":"2026-05-20","runtime":132,"status":"Released",
			"genres":[{"name":"Action"},{"name":"Adventure"}],
			"productionCompanies":[{"name":"Lucasfilm"},{"name":"Disney"}],
			"credits":{
				"cast":[{"name":"Pedro Pascal","character":"The Mandalorian","profilePath":"/pedro.jpg"}],
				"crew":[{"name":"Jon Favreau","job":"Director","profilePath":"/jon.jpg"}]
			}
		}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL, "api-key").MediaDetail(context.Background(), "movie", 1228710)
	if err != nil {
		t.Fatalf("MediaDetail() error = %v", err)
	}
	if detail.Title != "The Mandalorian and Grogu" || detail.Year != 2026 || detail.RuntimeMinutes != 132 {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.Status != "Veröffentlicht" || detail.ReleaseDate != "2026-05-20" {
		t.Fatalf("status = %q, releaseDate = %q", detail.Status, detail.ReleaseDate)
	}
	if len(detail.Studios) != 2 || detail.Studios[0] != "Lucasfilm" || detail.Studios[1] != "Disney" {
		t.Fatalf("studios = %#v", detail.Studios)
	}
	if len(detail.Genres) != 2 || detail.Genres[0] != "Action" {
		t.Fatalf("genres = %#v", detail.Genres)
	}
	if len(detail.Cast) != 1 || detail.Cast[0].Name != "Pedro Pascal" || detail.Cast[0].Role != "The Mandalorian" {
		t.Fatalf("cast = %#v", detail.Cast)
	}
	if len(detail.Crew) != 1 || detail.Crew[0].Name != "Jon Favreau" || detail.Crew[0].Role != "Director" {
		t.Fatalf("crew = %#v", detail.Crew)
	}
	if detail.PosterURL != "https://image.tmdb.org/t/p/w300/poster.jpg" {
		t.Fatalf("PosterURL = %q", detail.PosterURL)
	}
}

func TestMediaDetailParsesTvShowSeasonsAndSkipsSpecials(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/tv/12345" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"Severance","firstAirDate":"2022-02-18","seasons":[
			{"seasonNumber":0,"episodeCount":3},
			{"seasonNumber":1,"episodeCount":9},
			{"seasonNumber":2,"episodeCount":10}
		]}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL, "api-key").MediaDetail(context.Background(), "tv", 12345)
	if err != nil {
		t.Fatalf("MediaDetail() error = %v", err)
	}
	if detail.Title != "Severance" || detail.Year != 2022 {
		t.Fatalf("detail = %#v", detail)
	}
	if len(detail.Seasons) != 2 || detail.Seasons[0].SeasonNumber != 1 || detail.Seasons[1].EpisodeCount != 10 {
		t.Fatalf("Seasons = %#v, want season 0 (Specials) skipped", detail.Seasons)
	}
	if detail.MediaStatus != 0 {
		t.Fatalf("MediaStatus = %d, want 0 for a title with no mediaInfo", detail.MediaStatus)
	}
}

func TestMediaDetailReadsMediaStatusWhenPresent(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"title":"Dune","mediaInfo":{"status":5}}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL, "api-key").MediaDetail(context.Background(), "movie", 1)
	if err != nil {
		t.Fatalf("MediaDetail() error = %v", err)
	}
	if detail.MediaStatus != 5 {
		t.Fatalf("MediaStatus = %d, want 5", detail.MediaStatus)
	}
}

func TestMediaDetailMarksAvailableSeasonsFromMediaInfo(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"The Sopranos","firstAirDate":"1999-01-10","seasons":[
			{"seasonNumber":1,"episodeCount":13},
			{"seasonNumber":2,"episodeCount":13},
			{"seasonNumber":3,"episodeCount":13}
		],"mediaInfo":{"status":4,"seasons":[
			{"seasonNumber":1,"status":5},
			{"seasonNumber":2,"status":2},
			{"seasonNumber":3,"status":4}
		]}}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL, "api-key").MediaDetail(context.Background(), "tv", 12345)
	if err != nil {
		t.Fatalf("MediaDetail() error = %v", err)
	}
	if detail.MediaStatus != 4 {
		t.Fatalf("MediaStatus = %d, want 4 (partially available)", detail.MediaStatus)
	}
	if len(detail.Seasons) != 3 {
		t.Fatalf("Seasons = %#v", detail.Seasons)
	}
	if !detail.Seasons[0].Available {
		t.Fatalf("season 1 = %#v, want Available (status 5 in mediaInfo)", detail.Seasons[0])
	}
	if detail.Seasons[1].Available {
		t.Fatalf("season 2 = %#v, want not Available (status 2 in mediaInfo)", detail.Seasons[1])
	}
	if detail.Seasons[0].Requested {
		t.Fatalf("season 1 = %#v, want not Requested (status 5 in mediaInfo)", detail.Seasons[0])
	}
	if !detail.Seasons[1].Requested {
		t.Fatalf("season 2 = %#v, want Requested (status 2 pending in mediaInfo)", detail.Seasons[1])
	}
	if detail.Seasons[2].Available {
		t.Fatalf("season 3 = %#v, want not Available (status 4 partially available in mediaInfo)", detail.Seasons[2])
	}
	if !detail.Seasons[2].Requested {
		t.Fatalf("season 3 = %#v, want Requested (status 4 partially available means already spoken for)", detail.Seasons[2])
	}
}

func TestMediaDetailReturnsErrorWhenClientIsNil(t *testing.T) {
	var client *Client
	if _, err := client.MediaDetail(context.Background(), "movie", 1); err == nil {
		t.Fatal("MediaDetail() error = nil, want error for unconfigured Seerr")
	}
}
