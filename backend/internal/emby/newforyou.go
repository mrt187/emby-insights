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

const (
	newForYouWindow = 14 * 24 * time.Hour
	newForYouLimit  = 15
)

type NewForYouItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PosterURL   string `json:"posterUrl"`
	DateCreated string `json:"dateCreated"`
}

type NewForYouReader interface {
	NewForYou(context.Context, string, []string) ([]NewForYouItem, error)
}

// NewForYou reads a user's unseen, recently added items directly through the
// Emby API, scoped to the given library IDs, and narrows them down to the
// last 14 days, since Items/Latest on its own does not support a date
// filter. Items/Latest only accepts a single ParentId, so libraries are
// queried one at a time and merged.
func (client *Client) NewForYou(ctx context.Context, userID string, libraryIDs []string) ([]NewForYouItem, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	cutoff := time.Now().UTC().Add(-newForYouWindow)
	var items []NewForYouItem
	for _, libraryID := range libraryIDs {
		latest, err := client.latestItems(ctx, userID, libraryID)
		if err != nil {
			return nil, err
		}
		for _, item := range latest {
			if item.DateCreated == "" {
				continue
			}
			added, err := time.Parse(time.RFC3339, item.DateCreated)
			if err != nil || added.Before(cutoff) {
				continue
			}
			var posterURL string
			if item.ImageTags.Primary != "" {
				posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.Id, item.ImageTags.Primary)
			}
			items = append(items, NewForYouItem{ID: item.Id, Title: item.Name, PosterURL: posterURL, DateCreated: item.DateCreated})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].DateCreated > items[j].DateCreated })
	if len(items) > newForYouLimit {
		items = items[:newForYouLimit]
	}
	return items, nil
}

type embyLatestItem struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	DateCreated string `json:"DateCreated"`
	ImageTags   struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

func (client *Client) latestItems(ctx context.Context, userID, libraryID string) ([]embyLatestItem, error) {
	query := url.Values{
		"ParentId":         {libraryID},
		"IsPlayed":         {"false"},
		"Limit":            {"50"},
		"Fields":           {"DateCreated"},
		"IncludeItemTypes": {"Movie,Series"},
		"GroupItems":       {"false"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items/Latest?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby latest items: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby latest items returned %s", response.Status)
	}

	var latest []embyLatestItem
	if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
		return nil, fmt.Errorf("decode Emby latest items: %w", err)
	}
	return latest, nil
}
