package seerr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientReturnsNilWhenUnconfigured(t *testing.T) {
	if client := NewClient("", ""); client != nil {
		t.Fatalf("NewClient() = %#v, want nil", client)
	}
	if client := NewClient("https://seerr", ""); client != nil {
		t.Fatalf("NewClient() with missing api key = %#v, want nil", client)
	}
}

func TestRequestsReturnsNilWhenClientIsNil(t *testing.T) {
	var client *Client
	requests, err := client.Requests(context.Background(), "emby-user-1")
	if err != nil || requests != nil {
		t.Fatalf("Requests() = %#v, %v, want nil, nil", requests, err)
	}
}

func TestRequestsFiltersAvailableAndDeclined(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Api-Key") != "api-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Api-Key"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/user/jellyfin/emby-user-1":
			_, _ = writer.Write([]byte(`{"id":42}`))
		case "/api/v1/user/42/requests":
			_, _ = writer.Write([]byte(`{"results":[
				{"id":1,"status":1,"media":{"mediaType":"movie","tmdbId":100,"status":2}},
				{"id":2,"status":3,"media":{"mediaType":"movie","tmdbId":200,"status":2}},
				{"id":3,"status":2,"media":{"mediaType":"tv","tmdbId":300,"status":5}}
			]}`))
		case "/api/v1/movie/100":
			_, _ = writer.Write([]byte(`{"title":"Dune: Part Three","posterPath":"/dune.jpg"}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	requests, err := NewClient(testServer.URL, "api-key").Requests(context.Background(), "emby-user-1")
	if err != nil {
		t.Fatalf("Requests() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %#v, want exactly the pending, unavailable entry", requests)
	}
	if requests[0].Title != "Dune: Part Three" || requests[0].Status != "Angefragt" {
		t.Fatalf("requests[0] = %#v", requests[0])
	}
	if requests[0].PosterURL != "https://image.tmdb.org/t/p/w300/dune.jpg" {
		t.Fatalf("PosterURL = %q", requests[0].PosterURL)
	}
}

func TestRequestsReturnsNilWhenSeerrUserUnknown(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	requests, err := NewClient(testServer.URL, "api-key").Requests(context.Background(), "unlinked-user")
	if err != nil || requests != nil {
		t.Fatalf("Requests() = %#v, %v, want nil, nil", requests, err)
	}
}

func TestRequestStatsReadsTotalRequestCount(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/user/jellyfin/emby-user-1":
			_, _ = writer.Write([]byte(`{"id":27}`))
		case "/api/v1/user/27":
			_, _ = writer.Write([]byte(`{"requestCount":302}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	stats, err := NewClient(testServer.URL, "api-key").RequestStats(context.Background(), "emby-user-1")
	if err != nil {
		t.Fatalf("RequestStats() error = %v", err)
	}
	if stats.Total != 302 {
		t.Fatalf("stats = %#v, want Total = 302", stats)
	}
}

func TestRequestStatsReturnsZeroWhenSeerrUserUnknown(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	stats, err := NewClient(testServer.URL, "api-key").RequestStats(context.Background(), "unlinked-user")
	if err != nil || stats.Total != 0 {
		t.Fatalf("RequestStats() = %#v, %v, want zero value, nil", stats, err)
	}
}

func TestRequestStatsReturnsZeroWhenClientIsNil(t *testing.T) {
	var client *Client
	stats, err := client.RequestStats(context.Background(), "emby-user-1")
	if err != nil || stats.Total != 0 {
		t.Fatalf("RequestStats() = %#v, %v, want zero value, nil", stats, err)
	}
}
