package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
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
	WatchedMovies(ctx context.Context, userID, period string) ([]WatchedItem, error)
	WatchedSeries(ctx context.Context, userID, period string) ([]WatchedItem, error)
}

// WatchedMovies reads movies Emby has marked fully played, narrowed to the
// given period by LastPlayedDate.
func (client *Client) WatchedMovies(ctx context.Context, userID, period string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, period, "Movie")
}

// WatchedSeries reads series Emby has marked fully played — Emby rolls a
// series' Played status up from its episodes, so this already means "every
// currently available episode has been watched".
func (client *Client) WatchedSeries(ctx context.Context, userID, period string) ([]WatchedItem, error) {
	return client.watchedItems(ctx, userID, period, "Series")
}

func (client *Client) watchedItems(ctx context.Context, userID, period, itemType string) ([]WatchedItem, error) {
	from, to, err := periodRange(period, time.Now())
	if err != nil {
		return nil, err
	}

	query := url.Values{
		"Filters":          {"IsPlayed"},
		"IncludeItemTypes": {itemType},
		"SortBy":           {"DatePlayed"},
		"SortOrder":        {"Descending"},
		"Fields":           {"Genres"},
		"Limit":            {"100"},
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

	items := make([]WatchedItem, 0, len(result.Items))
	for _, item := range result.Items {
		if item.UserData.LastPlayedDate == "" {
			continue
		}
		played, err := time.Parse(time.RFC3339, item.UserData.LastPlayedDate)
		if err != nil || played.Before(from) || played.After(to) {
			continue
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
			LastPlayedDate: item.UserData.LastPlayedDate,
		})
		if len(items) >= watchedItemsLimit {
			break
		}
	}
	return items, nil
}
