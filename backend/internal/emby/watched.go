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
	// BackdropURL and DateAdded are used by the frontend to pick a hero image
	// for the "most recently added" title in a watched list — omitted when
	// Emby has no backdrop for the item.
	BackdropURL string `json:"backdropUrl,omitempty"`
	DateAdded   string `json:"dateAdded,omitempty"`
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
		found, err := client.watchedItemsInLibrary(ctx, userID, itemType, libraryID, watchedItemsLimit)
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
		lastPlayedDate, err := client.lastPlayedDateFor(ctx, userID, itemType, item.Id)
		if err != nil {
			return nil, err
		}

		var posterURL string
		if item.ImageTags.Primary != "" {
			posterURL = ImageURL(item.Id, "Primary", item.ImageTags.Primary, 400)
		}
		var backdropURL string
		if len(item.BackdropImageTags) > 0 {
			backdropURL = ImageURL(item.Id, "Backdrop", item.BackdropImageTags[0], 1600)
		}

		items = append(items, WatchedItem{
			ID:             item.Id,
			Title:          item.Name,
			PosterURL:      posterURL,
			Genres:         item.Genres,
			LastPlayedDate: lastPlayedDate,
			BackdropURL:    backdropURL,
			DateAdded:      item.DateCreated,
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].LastPlayedDate > items[j].LastPlayedDate })
	if len(items) > watchedItemsLimit {
		items = items[:watchedItemsLimit]
	}
	return items, nil
}

type embyWatchedCandidate struct {
	Id                string   `json:"Id"`
	Name              string   `json:"Name"`
	Genres            []string `json:"Genres"`
	DateCreated       string   `json:"DateCreated"`
	BackdropImageTags []string `json:"BackdropImageTags"`
	ImageTags         struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

func (client *Client) watchedItemsInLibrary(ctx context.Context, userID, itemType, libraryID string, limit int) ([]embyWatchedCandidate, error) {
	query := url.Values{
		"ParentId":         {libraryID},
		"Filters":          {"IsPlayed"},
		"IncludeItemTypes": {itemType},
		"SortBy":           {"DatePlayed"},
		"SortOrder":        {"Descending"},
		"Fields":           {"Genres,DateCreated,BackdropImageTags"},
		"Limit":            {strconv.Itoa(limit)},
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

// lastPlayedDateFor resolves the "last played" date for an item, accounting
// for a real Emby quirk: a series' own UserData never carries
// LastPlayedDate (only Played and UnplayedItemCount roll up from its
// episodes) — it has to be read off the most recently played episode
// instead. Movies (and everything else) carry it directly.
func (client *Client) lastPlayedDateFor(ctx context.Context, userID, itemType, itemID string) (string, error) {
	if itemType == "Series" {
		return client.seriesLastPlayedDate(ctx, userID, itemID)
	}
	return client.itemLastPlayedDate(ctx, userID, itemID)
}

func (client *Client) seriesLastPlayedDate(ctx context.Context, userID, seriesID string) (string, error) {
	query := url.Values{
		"UserId":    {userID},
		"Filters":   {"IsPlayed"},
		"SortBy":    {"DatePlayed"},
		"SortOrder": {"Descending"},
		"Limit":     {"1"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Shows/"+seriesID+"/Episodes?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call Emby series episodes: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Emby series episodes returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			Id string `json:"Id"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Emby series episodes: %w", err)
	}
	if len(result.Items) == 0 {
		return "", nil
	}
	return client.itemLastPlayedDate(ctx, userID, result.Items[0].Id)
}
