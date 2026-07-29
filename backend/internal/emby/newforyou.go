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
	NewForYou(context.Context, string) ([]NewForYouItem, error)
}

// NewForYou reads a user's unseen, recently added items directly through the
// Emby API and narrows them down to the last 14 days, since Items/Latest on
// its own does not support a date filter.
func (client *Client) NewForYou(ctx context.Context, userID string) ([]NewForYouItem, error) {
	query := url.Values{
		"IsPlayed":         {"false"},
		"Limit":            {"100"},
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

	var latest []struct {
		Id          string `json:"Id"`
		Name        string `json:"Name"`
		DateCreated string `json:"DateCreated"`
		ImageTags   struct {
			Primary string `json:"Primary"`
		} `json:"ImageTags"`
	}
	if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
		return nil, fmt.Errorf("decode Emby latest items: %w", err)
	}

	cutoff := time.Now().UTC().Add(-newForYouWindow)
	var items []NewForYouItem
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

	sort.Slice(items, func(i, j int) bool { return items[i].DateCreated > items[j].DateCreated })
	if len(items) > newForYouLimit {
		items = items[:newForYouLimit]
	}
	return items, nil
}
