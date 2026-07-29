package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

const upcomingWindow = 28 * 24 * time.Hour

type UpcomingItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	PosterURL    string `json:"posterUrl"`
	PremiereDate string `json:"premiereDate"`
}

type UpcomingReader interface {
	Upcoming(context.Context, []string) ([]UpcomingItem, error)
}

// Upcoming reads the configured ComingSoon libraries directly through the
// Emby API: ComingSoon creates a normal library per list and stores the
// release date as the item's PremiereDate, so no separate Radarr/Sonarr
// connection is needed.
func (client *Client) Upcoming(ctx context.Context, libraryIDs []string) ([]UpcomingItem, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	horizon := now.Add(upcomingWindow)

	var items []UpcomingItem
	for _, libraryID := range libraryIDs {
		libraryItems, err := client.libraryItems(ctx, libraryID)
		if err != nil {
			return nil, err
		}
		for _, item := range libraryItems {
			if item.PremiereDate == "" {
				continue
			}
			premiere, err := time.Parse(time.RFC3339, item.PremiereDate)
			if err != nil {
				continue
			}
			if premiere.Before(now) || premiere.After(horizon) {
				continue
			}
			var posterURL string
			if item.ImageTags.Primary != "" {
				posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.Id, item.ImageTags.Primary)
			}
			items = append(items, UpcomingItem{ID: item.Id, Title: item.Name, PosterURL: posterURL, PremiereDate: item.PremiereDate})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].PremiereDate < items[j].PremiereDate })
	return items, nil
}

type embyLibraryItem struct {
	Id           string `json:"Id"`
	Name         string `json:"Name"`
	PremiereDate string `json:"PremiereDate"`
	ImageTags    struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

func (client *Client) libraryItems(ctx context.Context, libraryID string) ([]embyLibraryItem, error) {
	query := url.Values{
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"IncludeItemTypes": {"Movie,Series"},
		"Fields":           {"PremiereDate"},
		"SortBy":           {"PremiereDate"},
		"SortOrder":        {"Ascending"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Items?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby library items: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby library items returned %s", response.Status)
	}

	var result struct {
		Items []embyLibraryItem `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby library items: %w", err)
	}
	return result.Items, nil
}
