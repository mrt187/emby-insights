package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const watchedItemsLimit = 24

type WatchedItem struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	PosterURL      string   `json:"posterUrl"`
	Genres         []string `json:"genres"`
	LastPlayedDate string   `json:"lastPlayedDate"`
}

type WatchedReader interface {
	WatchedMovies(ctx context.Context, userID string) ([]WatchedItem, error)
	WatchedSeries(ctx context.Context, userID string) ([]WatchedItem, error)
}

// WatchedMovies reads all movies Emby has marked fully played.
func (client *Client) WatchedMovies(ctx context.Context, userID string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, "Movie")
}

// WatchedSeries reads all series Emby has marked fully played — Emby rolls a
// series' Played status up from its episodes, so this already means "every
// currently available episode has been watched".
func (client *Client) WatchedSeries(ctx context.Context, userID string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, "Series")
}

func (client *Client) watchedItems(ctx context.Context, userID, itemType string) ([]WatchedItem, error) {
	query := url.Values{
		"Filters":          {"IsPlayed"},
		"IncludeItemTypes": {itemType},
		"SortBy":           {"DatePlayed"},
		"SortOrder":        {"Descending"},
		"Fields":           {"Genres"},
		"Limit":            {strconv.Itoa(watchedItemsLimit)},
		"Recursive":        {"true"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby watched items: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby watched items returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			Id        string   `json:"Id"`
			Name      string   `json:"Name"`
			Genres    []string `json:"Genres"`
			ImageTags struct {
				Primary string `json:"Primary"`
			} `json:"ImageTags"`
			UserData struct {
				LastPlayedDate string `json:"LastPlayedDate"`
			} `json:"UserData"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby watched items: %w", err)
	}

	// The bulk list endpoint omits UserData.LastPlayedDate on this Emby
	// version (only the single-item endpoint returns it), so it is looked up
	// individually per item — bounded by watchedItemsLimit above.
	items := make([]WatchedItem, 0, len(result.Items))
	for _, item := range result.Items {
		lastPlayedDate, err := client.itemLastPlayedDate(ctx, userID, item.Id)
		if err != nil {
			return nil, err
		}

		var posterURL string
		if item.ImageTags.Primary != "" {
			posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.Id, item.ImageTags.Primary)
		}

		items = append(items, WatchedItem{
			ID:             item.Id,
			Title:          item.Name,
			PosterURL:      posterURL,
			Genres:         item.Genres,
			LastPlayedDate: lastPlayedDate,
		})
	}
	return items, nil
}

func (client *Client) itemLastPlayedDate(ctx context.Context, userID, itemID string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items/"+itemID, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call Emby item detail: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Emby item detail returned %s", response.Status)
	}

	var result struct {
		UserData struct {
			LastPlayedDate string `json:"LastPlayedDate"`
		} `json:"UserData"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Emby item detail: %w", err)
	}
	return result.UserData.LastPlayedDate, nil
}
