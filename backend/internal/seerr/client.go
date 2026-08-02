// Package seerr talks to a Jellyseerr/Overseerr instance to read a user's
// own open media requests. The exact response shape is documented but has
// not been verified against a live instance yet (see docs/INTEGRATIONS.md);
// status labels in particular may need adjusting once real data flows in.
package seerr

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mrt187/EmbyInsights/internal/artwork"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const posterBaseURL = "https://image.tmdb.org/t/p/w300"

type Request struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	PosterURL string `json:"posterUrl"`
	Status    string `json:"status"`
	TmdbID    string `json:"tmdbId"`
	MediaType string `json:"mediaType"`
	// AvailableSince is only set by AvailableRequests — the raw ISO
	// timestamp at which Seerr recorded the title as added to the library.
	AvailableSince string `json:"availableSince,omitempty"`
}

type RequestsReader interface {
	Requests(context.Context, string) ([]Request, error)
}

type AvailableRequestsReader interface {
	AvailableRequests(ctx context.Context, embyUserID string, since time.Time) ([]Request, error)
}

type RequestStats struct {
	Total int `json:"total"`
}

type RequestStatsReader interface {
	RequestStats(ctx context.Context, embyUserID string) (RequestStats, error)
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient returns nil when Seerr is not configured. Its methods are safe
// to call on a nil receiver and simply report no requests, so callers do not
// need to special-case an unconfigured integration.
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" || apiKey == "" {
		return nil
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (client *Client) Requests(ctx context.Context, embyUserID string) ([]Request, error) {
	if client == nil {
		return nil, nil
	}

	seerrUserID, ok, err := client.resolveUserID(ctx, embyUserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	entries, err := client.userRequests(ctx, seerrUserID)
	if err != nil {
		return nil, err
	}

	var requests []Request
	for _, entry := range entries {
		if entry.Media.Status == mediaStatusAvailable || entry.Status == requestStatusDeclined {
			continue
		}
		detail, err := client.mediaDetail(ctx, entry.Media.MediaType, entry.Media.TmdbID)
		if err != nil {
			return nil, err
		}
		requests = append(requests, Request{
			ID:        strconv.Itoa(entry.ID),
			Title:     detail.title,
			PosterURL: detail.posterURL,
			Status:    statusLabel(entry.Status, entry.Media.Status),
			TmdbID:    strconv.Itoa(entry.Media.TmdbID),
			MediaType: entry.Media.MediaType,
		})
	}
	return requests, nil
}

// AvailableRequests reports the user's requests whose title has become
// available in the library since the given time — deliberately separate from
// Requests, which filters exactly these out because its "open requests" row
// is about what is still outstanding.
func (client *Client) AvailableRequests(ctx context.Context, embyUserID string, since time.Time) ([]Request, error) {
	if client == nil {
		return nil, nil
	}

	seerrUserID, ok, err := client.resolveUserID(ctx, embyUserID)
	if err != nil || !ok {
		return nil, err
	}

	entries, err := client.userRequests(ctx, seerrUserID)
	if err != nil {
		return nil, err
	}

	var requests []Request
	for _, entry := range entries {
		if entry.Media.Status != mediaStatusAvailable || entry.Media.MediaAddedAt == "" {
			continue
		}
		addedAt, err := time.Parse(time.RFC3339, entry.Media.MediaAddedAt)
		if err != nil || addedAt.Before(since) {
			continue
		}
		detail, err := client.mediaDetail(ctx, entry.Media.MediaType, entry.Media.TmdbID)
		if err != nil {
			return nil, err
		}
		requests = append(requests, Request{
			ID:             strconv.Itoa(entry.ID),
			Title:          detail.title,
			PosterURL:      detail.posterURL,
			Status:         "Jetzt verfügbar",
			TmdbID:         strconv.Itoa(entry.Media.TmdbID),
			MediaType:      entry.Media.MediaType,
			AvailableSince: entry.Media.MediaAddedAt,
		})
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].AvailableSince > requests[j].AvailableSince })
	return requests, nil
}

// RequestStats reads the total number of requests this user has ever made
// on Seerr — unlike Requests, this is not filtered to open/available ones.
func (client *Client) RequestStats(ctx context.Context, embyUserID string) (RequestStats, error) {
	if client == nil {
		return RequestStats{}, nil
	}

	seerrUserID, ok, err := client.resolveUserID(ctx, embyUserID)
	if err != nil {
		return RequestStats{}, err
	}
	if !ok {
		return RequestStats{}, nil
	}

	var result struct {
		RequestCount int `json:"requestCount"`
	}
	if _, err := client.get(ctx, fmt.Sprintf("/api/v1/user/%d", seerrUserID), &result); err != nil {
		return RequestStats{}, err
	}
	return RequestStats{Total: result.RequestCount}, nil
}

func (client *Client) resolveUserID(ctx context.Context, embyUserID string) (int, bool, error) {
	var result struct {
		ID int `json:"id"`
	}
	found, err := client.get(ctx, "/api/v1/user/jellyfin/"+embyUserID, &result)
	if err != nil || !found {
		return 0, false, err
	}
	return result.ID, true, nil
}

const (
	requestStatusDeclined = 3
	mediaStatusAvailable  = 5
)

type requestEntry struct {
	ID     int `json:"id"`
	Status int `json:"status"`
	Media  struct {
		MediaType    string `json:"mediaType"`
		TmdbID       int    `json:"tmdbId"`
		Status       int    `json:"status"`
		MediaAddedAt string `json:"mediaAddedAt"`
	} `json:"media"`
}

func (client *Client) userRequests(ctx context.Context, seerrUserID int) ([]requestEntry, error) {
	var result struct {
		Results []requestEntry `json:"results"`
	}
	path := fmt.Sprintf("/api/v1/user/%d/requests?take=50", seerrUserID)
	if _, err := client.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

type mediaDetail struct {
	title     string
	posterURL string
}

func (client *Client) mediaDetail(ctx context.Context, mediaType string, tmdbID int) (mediaDetail, error) {
	var result struct {
		Title      string `json:"title"` // present on movies
		Name       string `json:"name"`  // present on tv shows
		PosterPath string `json:"posterPath"`
	}
	path := fmt.Sprintf("/api/v1/%s/%d", mediaType, tmdbID)
	if _, err := client.get(ctx, path, &result); err != nil {
		return mediaDetail{}, err
	}
	title := result.Title
	if title == "" {
		title = result.Name
	}
	var posterURL string
	if result.PosterPath != "" {
		posterURL = artwork.ProxyURL(posterBaseURL + result.PosterPath)
	}
	return mediaDetail{title: title, posterURL: posterURL}, nil
}

// get decodes a successful JSON response into target and reports false
// (without an error) on a 404, since a missing Seerr user or media entry is
// an expected, non-fatal outcome.
func (client *Client) get(ctx context.Context, path string, target any) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("X-Api-Key", client.apiKey)
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("call Seerr %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Seerr %s returned %s", path, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return false, fmt.Errorf("decode Seerr %s: %w", path, err)
	}
	return true, nil
}

func statusLabel(requestStatus, mediaStatus int) string {
	switch {
	case requestStatus == 1:
		return "Angefragt"
	case mediaStatus == 3:
		return "In Bearbeitung"
	case mediaStatus == 4:
		return "Teilweise verfügbar"
	case requestStatus == 2:
		return "Genehmigt"
	default:
		return "Wird gesucht"
	}
}
