package seerr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRequestSendsAllSeasonsWhenNoneSpecified(t *testing.T) {
	var received map[string]any
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/user/jellyfin/emby-user-1":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":27}`))
		case "/api/v1/request":
			if request.Method != http.MethodPost {
				t.Fatalf("method = %q", request.Method)
			}
			if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	err := NewClient(testServer.URL, "api-key").CreateRequest(context.Background(), "emby-user-1", "tv", 94997, nil)
	if err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	if received["mediaType"] != "tv" || received["seasons"] != "all" {
		t.Fatalf("body = %#v", received)
	}
	if userID, ok := received["userId"].(float64); !ok || int(userID) != 27 {
		t.Fatalf("userId = %#v, want 27", received["userId"])
	}
}

func TestCreateRequestSendsSpecificSeasons(t *testing.T) {
	var received map[string]any
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/user/jellyfin/emby-user-1":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":27}`))
		case "/api/v1/request":
			_ = json.NewDecoder(request.Body).Decode(&received)
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	if err := NewClient(testServer.URL, "api-key").CreateRequest(context.Background(), "emby-user-1", "tv", 94997, []int{1, 2}); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	seasons, ok := received["seasons"].([]any)
	if !ok || len(seasons) != 2 {
		t.Fatalf("seasons = %#v", received["seasons"])
	}
}

func TestCreateRequestOmitsSeasonsForMovies(t *testing.T) {
	var received map[string]any
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/user/jellyfin/emby-user-1":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":27}`))
		case "/api/v1/request":
			_ = json.NewDecoder(request.Body).Decode(&received)
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	if err := NewClient(testServer.URL, "api-key").CreateRequest(context.Background(), "emby-user-1", "movie", 1228710, nil); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	if _, hasSeasons := received["seasons"]; hasSeasons {
		t.Fatalf("body = %#v, want no seasons field for a movie request", received)
	}
}

func TestCreateRequestFailsWithoutLinkedSeerrUser(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	err := NewClient(testServer.URL, "api-key").CreateRequest(context.Background(), "unlinked-user", "movie", 1, nil)
	if err == nil {
		t.Fatal("CreateRequest() error = nil, want error when no Seerr user is linked")
	}
}

func TestCreateRequestReturnsErrorWhenClientIsNil(t *testing.T) {
	var client *Client
	if err := client.CreateRequest(context.Background(), "emby-user-1", "movie", 1, nil); err == nil {
		t.Fatal("CreateRequest() error = nil, want error for unconfigured Seerr")
	}
}
