// Package omdb reads IMDb and Rotten Tomatoes ratings from the OMDb API.
// TMDB's own API only exposes its own community score, not third-party
// ratings — OMDb is the standard way self-hosted media dashboards fill
// that gap, keyed by IMDb id rather than TMDB id.
package omdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type Ratings struct {
	ImdbRating           string
	RottenTomatoesRating string
}

const defaultBaseURL = "https://www.omdbapi.com/"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{apiKey: apiKey, baseURL: defaultBaseURL, httpClient: &http.Client{Timeout: 8 * time.Second}}
}

// Ratings looks up one title's IMDb and Rotten Tomatoes scores. OMDb only
// supports key-in-query-string auth (no header alternative, unlike TMDB) —
// this is a decoration on the media-detail screen, not a page the user is
// blocked on, so a nil client, empty imdbID, or any request/parse/lookup
// failure returns a zero Ratings and no error rather than failing the
// caller.
func (client *Client) Ratings(ctx context.Context, imdbID string) (Ratings, error) {
	if client == nil || imdbID == "" {
		return Ratings{}, nil
	}

	endpoint := client.baseURL + "?" + url.Values{
		"apikey": {client.apiKey},
		"i":      {imdbID},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Ratings{}, nil
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return Ratings{}, nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Ratings{}, nil
	}

	var result struct {
		Response    string `json:"Response"`
		ImdbRating  string `json:"imdbRating"`
		RatingsList []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Ratings{}, nil
	}
	if result.Response != "True" {
		return Ratings{}, nil
	}

	var ratings Ratings
	if result.ImdbRating != "" && result.ImdbRating != "N/A" {
		ratings.ImdbRating = result.ImdbRating
	}
	for _, entry := range result.RatingsList {
		if entry.Source == "Rotten Tomatoes" {
			ratings.RottenTomatoesRating = entry.Value
		}
	}
	return ratings, nil
}
