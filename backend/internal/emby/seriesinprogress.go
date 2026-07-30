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

const seriesInProgressLimit = 24

// SeriesProgress is a series the user has started but not finished — some
// episodes watched, some not, distinct from WatchedSeries (fully played)
// and ContinueWatching (a specific in-progress episode's playback position).
type SeriesProgress struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PosterURL       string `json:"posterUrl"`
	WatchedEpisodes int    `json:"watchedEpisodes"`
	TotalEpisodes   int    `json:"totalEpisodes"`
}

type SeriesInProgressReader interface {
	SeriesInProgress(ctx context.Context, userID string, libraryIDs []string) ([]SeriesProgress, error)
}

// SeriesInProgress lists series with some, but not all, episodes watched.
// Emby has no server-side filter for "partially played" at the series
// level (Filters=IsPlayed only means "fully played", rolled up from
// episodes) — every series in the scoped libraries is fetched with its
// recursive episode/unplayed counts and filtered client-side instead, the
// same way MediaDetail already derives per-series progress.
func (client *Client) SeriesInProgress(ctx context.Context, userID string, libraryIDs []string) ([]SeriesProgress, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	var candidates []embySeriesCandidate
	for _, libraryID := range libraryIDs {
		found, err := client.seriesInLibrary(ctx, userID, libraryID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, found...)
	}

	items := make([]SeriesProgress, 0, len(candidates))
	for _, item := range candidates {
		watched := item.RecursiveItemCount - item.UserData.UnplayedItemCount
		if watched <= 0 || item.UserData.UnplayedItemCount <= 0 {
			continue
		}
		var posterURL string
		if item.ImageTags.Primary != "" {
			posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.Id, item.ImageTags.Primary)
		}
		items = append(items, SeriesProgress{
			ID:              item.Id,
			Title:           item.Name,
			PosterURL:       posterURL,
			WatchedEpisodes: watched,
			TotalEpisodes:   item.RecursiveItemCount,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return float64(items[i].WatchedEpisodes)/float64(items[i].TotalEpisodes) > float64(items[j].WatchedEpisodes)/float64(items[j].TotalEpisodes)
	})
	if len(items) > seriesInProgressLimit {
		items = items[:seriesInProgressLimit]
	}
	return items, nil
}

type embySeriesCandidate struct {
	Id                 string `json:"Id"`
	Name               string `json:"Name"`
	RecursiveItemCount int    `json:"RecursiveItemCount"`
	ImageTags          struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
	UserData struct {
		UnplayedItemCount int `json:"UnplayedItemCount"`
	} `json:"UserData"`
}

func (client *Client) seriesInLibrary(ctx context.Context, userID, libraryID string) ([]embySeriesCandidate, error) {
	query := url.Values{
		"ParentId":         {libraryID},
		"IncludeItemTypes": {"Series"},
		"Recursive":        {"true"},
		"Fields":           {"RecursiveItemCount"},
		"Limit":            {strconv.Itoa(seriesInProgressLimit * 4)},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby series list: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby series list returned %s", response.Status)
	}

	var result struct {
		Items []embySeriesCandidate `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby series list: %w", err)
	}
	return result.Items, nil
}
