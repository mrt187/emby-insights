package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Library is one Emby virtual folder (a top-level content library), listed
// for the Verwaltung admin UI so the operator can pick libraries by name
// instead of looking up their IDs manually.
type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Libraries lists every Emby library the server-wide admin API key can see.
func (client *Client) Libraries(ctx context.Context) ([]Library, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Library/VirtualFolders", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby virtual folders: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby virtual folders returned %s", response.Status)
	}

	var raw []struct {
		Name           string `json:"Name"`
		ItemID         string `json:"ItemId"`
		CollectionType string `json:"CollectionType"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode Emby virtual folders: %w", err)
	}

	libraries := make([]Library, 0, len(raw))
	for _, entry := range raw {
		libraries = append(libraries, Library{ID: entry.ItemID, Name: entry.Name})
	}
	return libraries, nil
}
