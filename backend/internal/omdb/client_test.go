package omdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRatingsParsesImdbAndRottenTomatoes(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("i") != "tt0141842" {
			t.Fatalf("i = %q", request.URL.Query().Get("i"))
		}
		if request.URL.Query().Get("apikey") != "test-key" {
			t.Fatalf("apikey = %q", request.URL.Query().Get("apikey"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Response":"True","imdbRating":"9.2","Ratings":[
			{"Source":"Internet Movie Database","Value":"9.2/10"},
			{"Source":"Rotten Tomatoes","Value":"92%"}
		]}`))
	}))
	defer testServer.Close()

	client := &Client{apiKey: "test-key", baseURL: testServer.URL, httpClient: testServer.Client()}
	ratings, err := client.Ratings(context.Background(), "tt0141842")
	if err != nil {
		t.Fatalf("Ratings() error = %v", err)
	}
	if ratings.ImdbRating != "9.2" {
		t.Fatalf("ImdbRating = %q, want 9.2", ratings.ImdbRating)
	}
	if ratings.RottenTomatoesRating != "92%" {
		t.Fatalf("RottenTomatoesRating = %q, want 92%%", ratings.RottenTomatoesRating)
	}
}

func TestRatingsReturnsZeroValueWhenNotFound(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Response":"False","Error":"Incorrect IMDb ID."}`))
	}))
	defer testServer.Close()

	client := &Client{apiKey: "test-key", baseURL: testServer.URL, httpClient: testServer.Client()}
	ratings, err := client.Ratings(context.Background(), "tt0000000")
	if err != nil {
		t.Fatalf("Ratings() error = %v", err)
	}
	if ratings != (Ratings{}) {
		t.Fatalf("ratings = %#v, want zero value", ratings)
	}
}

func TestRatingsNilClientOrEmptyID(t *testing.T) {
	var client *Client
	if ratings, err := client.Ratings(context.Background(), "tt0141842"); err != nil || ratings != (Ratings{}) {
		t.Fatalf("nil client: ratings = %#v, err = %v", ratings, err)
	}
	client = &Client{apiKey: "test-key", baseURL: defaultBaseURL, httpClient: http.DefaultClient}
	if ratings, err := client.Ratings(context.Background(), ""); err != nil || ratings != (Ratings{}) {
		t.Fatalf("empty imdbID: ratings = %#v, err = %v", ratings, err)
	}
}
