package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
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
	WatchedMovies(ctx context.Context, userID string, libraryIDs []string) ([]WatchedItem, error)
	WatchedSeries(ctx context.Context, userID string, libraryIDs []string) ([]WatchedItem, error)
}

// WatchedMovies reads all movies Emby has marked fully played, scoped to the
// given library IDs.
func (client *Client) WatchedMovies(ctx context.Context, userID string, libraryIDs []string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, "Movie", libraryIDs)
}

// WatchedSeries reads all series Emby has marked fully played, scoped to the
// given library IDs — Emby rolls a series' Played status up from its
// episodes, so this already means "every currently available episode has
// been watched".
func (client *Client) WatchedSeries(ctx context.Context, userID string, libraryIDs []string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, "Series", libraryIDs)
}

func (client *Client) watchedItems(ctx context.Context, userID, itemType string, libraryIDs []string) ([]WatchedItem, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	var candidates []embyWatchedCandidate
	for _, libraryID := range libraryIDs {
		found, err := client.watchedItemsInLibrary(ctx, userID, itemType, libraryID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, found...)
	}

	// The bulk list endpoint omits UserData.LastPlayedDate on this Emby
	// version (only the single-item endpoint returns it), so it is looked up
	// individually per candidate, then the combined, multi-library result is
	// re-sorted by that date before capping it.
	items := make([]WatchedItem, 0, len(candidates))
	for _, item := range candidates {
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

	sort.Slice(items, func(i, j int) bool { return items[i].LastPlayedDate > items[j].LastPlayedDate })
	if len(items) > watchedItemsLimit {
		items = items[:watchedItemsLimit]
	}
	return items, nil
}

type embyWatchedCandidate struct {
	Id        string   `json:"Id"`
	Name      string   `json:"Name"`
	Genres    []string `json:"Genres"`
	ImageTags struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

func (client *Client) watchedItemsInLibrary(ctx context.Context, userID, itemType, libraryID string) ([]embyWatchedCandidate, error) {
	query := url.Values{
		"ParentId":         {libraryID},
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
		Items []embyWatchedCandidate `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby watched items: %w", err)
	}
	return result.Items, nil
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
