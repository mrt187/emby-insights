package seerr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type RequestCreator interface {
	CreateRequest(ctx context.Context, embyUserID, mediaType string, tmdbID int, seasons []int) error
}

// CreateRequest submits a new Seerr request for the given TMDB title, on
// behalf of the Emby user's linked Seerr account, using whichever Radarr/
// Sonarr server and quality profile Seerr already has marked as default —
// this app never exposes a profile picker. For a "tv" request, seasons
// lists the season numbers to request; a nil/empty slice requests every
// season.
func (client *Client) CreateRequest(ctx context.Context, embyUserID, mediaType string, tmdbID int, seasons []int) error {
	if client == nil {
		return fmt.Errorf("Seerr is not configured")
	}

	seerrUserID, ok, err := client.resolveUserID(ctx, embyUserID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no Seerr account is linked to this Emby user")
	}

	body := map[string]any{
		"mediaType": mediaType,
		"mediaId":   tmdbID,
		"userId":    seerrUserID,
	}
	if mediaType == "tv" {
		if len(seasons) > 0 {
			body["seasons"] = seasons
		} else {
			body["seasons"] = "all"
		}
	}

	return client.post(ctx, "/api/v1/request", body)
}

func (client *Client) post(ctx context.Context, path string, body any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("X-Api-Key", client.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call Seerr %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Seerr %s returned %s", path, response.Status)
	}
	return nil
}
