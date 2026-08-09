// Package omdb reads IMDb and Rotten Tomatoes ratings from the OMDb API.
// TMDB's own API only exposes its own community score, not third-party
// ratings — OMDb is the standard way self-hosted media dashboards fill
// that gap, keyed by IMDb id rather than TMDB id.
package omdb

import (
	"context"
	"encoding/json"
	"fmt"
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
// supports key-in-query-string auth (no header alternative, unlike TMDB).
//
// The ratings are decoration on the media-detail screen, so callers are
// expected to carry on when this fails — but it does report the failure
// rather than swallowing it. Returning a zero Ratings and no error for
// every cause made an expired key, an exhausted daily quota and a title
// OMDb simply does not carry indistinguishable from each other and from
// "this film has no ratings", with nothing in the log either way.
//
// A title OMDb does not know is not a failure: that returns empty ratings
// and no error, so an unknown title logs nothing.
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
		return Ratings{}, fmt.Errorf("build OMDb request for %s: %w", imdbID, err)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return Ratings{}, fmt.Errorf("call OMDb for %s: %w", imdbID, err)
	}
	defer response.Body.Close()
	// 401 here is the usual one: OMDb answers it both for a bad key and for
	// "Request limit reached!" once the free tier's daily quota is spent.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Ratings{}, fmt.Errorf("OMDb returned %s for %s", response.Status, imdbID)
	}

	var result struct {
		Response    string `json:"Response"`
		Error       string `json:"Error"`
		ImdbRating  string `json:"imdbRating"`
		RatingsList []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Ratings{}, fmt.Errorf("decode OMDb response for %s: %w", imdbID, err)
	}
	if result.Response != "True" {
		if result.Error == "Movie not found!" || result.Error == "Incorrect IMDb ID." {
			return Ratings{}, nil
		}
		return Ratings{}, fmt.Errorf("OMDb refused %s: %s", imdbID, result.Error)
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
