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
			"voteAverage":7.3,"releaseDate":"2026-05-20","runtime":132,
			"genres":[{"name":"Action"},{"name":"Adventure"}],
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

func TestMediaDetailParsesTvShow(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/tv/12345" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"Severance","firstAirDate":"2022-02-18"}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL, "api-key").MediaDetail(context.Background(), "tv", 12345)
	if err != nil {
		t.Fatalf("MediaDetail() error = %v", err)
	}
	if detail.Title != "Severance" || detail.Year != 2022 {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestMediaDetailReturnsErrorWhenClientIsNil(t *testing.T) {
	var client *Client
	if _, err := client.MediaDetail(context.Background(), "movie", 1); err == nil {
		t.Fatal("MediaDetail() error = nil, want error for unconfigured Seerr")
	}
}
