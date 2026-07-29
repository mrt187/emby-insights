package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type PersonalWatchTime struct {
	WatchSeconds         int64  `json:"watchSeconds"`
	PreviousWatchSeconds int64  `json:"previousWatchSeconds"`
	CompletedMovies      int    `json:"completedMovies"`
	CompletedSeries      int    `json:"completedSeries"`
	FavouriteGenre       string `json:"favouriteGenre"`
	PeriodStartsAt       string `json:"periodStartsAt"`
	PeriodEndsAt         string `json:"periodEndsAt"`
}

type PersonalStatisticsReader interface {
	PersonalWatchTime(context.Context, string, string) (PersonalWatchTime, error)
}

type DeviceWatchTime struct {
	DeviceName   string `json:"deviceName"`
	WatchSeconds int64  `json:"watchSeconds"`
}

type DeviceStatisticsReader interface {
	DeviceWatchTimes(ctx context.Context, userID, period string) ([]DeviceWatchTime, error)
}

// DeviceWatchTimes reads per-device watch time for a period from the Emby
// Insights connector plugin — which device (TV, phone, browser, ...) the
// user watched on, sourced from Playback Reporting's own DeviceName column.
func (client *Client) DeviceWatchTimes(ctx context.Context, userID, period string) ([]DeviceWatchTime, error) {
	query := url.Values{"UserId": {userID}, "Period": {period}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/EmbyInsights/PersonalStats/Devices?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby Insights device statistics: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby Insights device statistics returned %s", response.Status)
	}

	var devices []DeviceWatchTime
	if err := json.NewDecoder(response.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("decode Emby Insights device statistics: %w", err)
	}
	return devices, nil
}

func (client *Client) PersonalWatchTime(ctx context.Context, userID, period string) (PersonalWatchTime, error) {
	query := url.Values{"UserId": {userID}, "Period": {period}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/EmbyInsights/PersonalStats?"+query.Encode(), nil)
	if err != nil {
		return PersonalWatchTime{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return PersonalWatchTime{}, fmt.Errorf("call Emby Insights personal statistics: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PersonalWatchTime{}, fmt.Errorf("Emby Insights personal statistics returned %s", response.Status)
	}

	var statistics PersonalWatchTime
	if err := json.NewDecoder(response.Body).Decode(&statistics); err != nil {
		return PersonalWatchTime{}, fmt.Errorf("decode Emby Insights personal statistics: %w", err)
	}
	return statistics, nil
}
