package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ContinueWatchingItem struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PosterURL       string `json:"posterUrl"`
	ProgressPercent int    `json:"progressPercent"`
}

type ContinueWatchingReader interface {
	ContinueWatching(context.Context, string) ([]ContinueWatchingItem, error)
}

// ContinueWatching reads Emby's own "resume" list — items with in-progress
// playback — for episodes and movies.
func (client *Client) ContinueWatching(ctx context.Context, userID string) ([]ContinueWatchingItem, error) {
	query := url.Values{
		"Limit":            {"12"},
		"Fields":           {"SeriesName,SeriesPrimaryImageTag"},
		"IncludeItemTypes": {"Movie,Episode"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items/Resume?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby resume items: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby resume items returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			Id           string `json:"Id"`
			Name         string `json:"Name"`
			SeriesName   string `json:"SeriesName"`
			RunTimeTicks int64  `json:"RunTimeTicks"`
			ImageTags    struct {
				Primary string `json:"Primary"`
			} `json:"ImageTags"`
			SeriesPrimaryImageTag string `json:"SeriesPrimaryImageTag"`
			SeriesId              string `json:"SeriesId"`
			UserData              struct {
				PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
			} `json:"UserData"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby resume items: %w", err)
	}

	items := make([]ContinueWatchingItem, 0, len(result.Items))
	for _, item := range result.Items {
		title := item.Name
		if item.SeriesName != "" {
			title = item.SeriesName + " – " + item.Name
		}

		var progress int
		if item.RunTimeTicks > 0 {
			progress = int(item.UserData.PlaybackPositionTicks * 100 / item.RunTimeTicks)
			if progress > 100 {
				progress = 100
			}
		}

		var posterURL string
		switch {
		case item.SeriesId != "" && item.SeriesPrimaryImageTag != "":
			posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.SeriesId, item.SeriesPrimaryImageTag)
		case item.ImageTags.Primary != "":
			posterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=400", client.baseURL, item.Id, item.ImageTags.Primary)
		}

		items = append(items, ContinueWatchingItem{ID: item.Id, Title: title, PosterURL: posterURL, ProgressPercent: progress})
	}
	return items, nil
}
