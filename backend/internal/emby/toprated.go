package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
)

const topRatedLimit = 15

type TopRatedItem struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	PosterURL       string  `json:"posterUrl"`
	CommunityRating float64 `json:"communityRating"`
}

type TopRatedReader interface {
	TopRated(ctx context.Context, userID string, libraryIDs []string) ([]TopRatedItem, error)
}

// TopRated reads the highest community-rated movies and series across the
// given libraries — the "Beliebt auf Emby" home row. Unrated items (rating
// 0, which is how Emby reports "no rating known" rather than omitting the
// field) are excluded, since a 0.0 would otherwise sort to the bottom but
// still clutter a "top rated" list with titles that aren't actually rated.
func (client *Client) TopRated(ctx context.Context, userID string, libraryIDs []string) ([]TopRatedItem, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	var items []TopRatedItem
	for _, libraryID := range libraryIDs {
		found, err := client.topRatedItemsInLibrary(ctx, userID, libraryID)
		if err != nil {
			return nil, err
		}
		for _, item := range found {
			if item.CommunityRating <= 0 {
				continue
			}
			var posterURL string
			if item.ImageTags.Primary != "" {
				posterURL = ImageURL(item.Id, "Primary", item.ImageTags.Primary, 400)
			}
			items = append(items, TopRatedItem{ID: item.Id, Title: item.Name, PosterURL: posterURL, CommunityRating: item.CommunityRating})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CommunityRating > items[j].CommunityRating })
	if len(items) > topRatedLimit {
		items = items[:topRatedLimit]
	}
	return items, nil
}

type embyTopRatedCandidate struct {
	Id              string  `json:"Id"`
	Name            string  `json:"Name"`
	CommunityRating float64 `json:"CommunityRating"`
	ImageTags       struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

func (client *Client) topRatedItemsInLibrary(ctx context.Context, userID, libraryID string) ([]embyTopRatedCandidate, error) {
	query := url.Values{
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"IncludeItemTypes": {"Movie,Series"},
		"SortBy":           {"CommunityRating"},
		"SortOrder":        {"Descending"},
		"Fields":           {"CommunityRating"},
		"Limit":            {"20"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby top rated items: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby top rated items returned %s", response.Status)
	}

	var result struct {
		Items []embyTopRatedCandidate `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby top rated items: %w", err)
	}
	return result.Items, nil
}
