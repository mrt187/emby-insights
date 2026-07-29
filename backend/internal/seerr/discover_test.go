package seerr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrendingDecodesMixedMediaTypes(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/discover/trending" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Api-Key") != "api-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Api-Key"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"results":[
			{"id":100,"mediaType":"movie","title":"Dune","posterPath":"/dune.jpg"},
			{"id":200,"mediaType":"tv","name":"Severance","posterPath":"/severance.jpg"}
		]}`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL, "api-key").Trending(context.Background())
	if err != nil {
		t.Fatalf("Trending() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Title != "Dune" || items[0].MediaType != "movie" || items[0].PosterURL != "https://image.tmdb.org/t/p/w300/dune.jpg" {
		t.Fatalf("items[0] = %#v", items[0])
	}
	if items[1].Title != "Severance" || items[1].MediaType != "tv" {
		t.Fatalf("items[1] = %#v", items[1])
	}
}

func TestPopularMoviesUsesCorrectPath(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/discover/movies" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"results":[]}`))
	}))
	defer testServer.Close()

	if _, err := NewClient(testServer.URL, "api-key").PopularMovies(context.Background()); err != nil {
		t.Fatalf("PopularMovies() error = %v", err)
	}
}

func TestDiscoverReturnsNilWhenClientIsNil(t *testing.T) {
	var client *Client
	items, err := client.Trending(context.Background())
	if err != nil || items != nil {
		t.Fatalf("Trending() = %#v, %v, want nil, nil", items, err)
	}
}
