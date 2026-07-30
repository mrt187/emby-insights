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

type WatchTimeRank struct {
	Rank int `json:"rank"`
}

type WatchTimeRankReader interface {
	WatchTimeRank(context.Context, string) (WatchTimeRank, error)
}

func (client *Client) WatchTimeRank(ctx context.Context, userID string) (WatchTimeRank, error) {
	query := url.Values{"UserId": {userID}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/EmbyInsights/PersonalStats/Rank?"+query.Encode(), nil)
	if err != nil {
		return WatchTimeRank{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return WatchTimeRank{}, fmt.Errorf("call Emby Insights watch-time rank: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return WatchTimeRank{}, fmt.Errorf("Emby Insights watch-time rank returned %s", response.Status)
	}

	var rank WatchTimeRank
	if err := json.NewDecoder(response.Body).Decode(&rank); err != nil {
		return WatchTimeRank{}, fmt.Errorf("decode Emby Insights watch-time rank: %w", err)
	}
	return rank, nil
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

type HourWatchTime struct {
	Hour         int   `json:"hour"`
	WatchSeconds int64 `json:"watchSeconds"`
}

type WeekdayWatchTime struct {
	// Weekday is 0=Monday..6=Sunday, already remapped by the plugin.
	Weekday      int   `json:"weekday"`
	WatchSeconds int64 `json:"watchSeconds"`
}

type LongestSession struct {
	ItemName     string `json:"itemName"`
	WatchSeconds int64  `json:"watchSeconds"`
	StartedAt    string `json:"startedAt"`
}

type MostActiveDay struct {
	Date         string `json:"date"`
	WatchSeconds int64  `json:"watchSeconds"`
}

// SessionStatisticsReader groups the personal-statistics endpoints that read
// directly off Playback Reporting's own session records (as opposed to
// PersonalWatchTime's coarser per-item completion data).
type SessionStatisticsReader interface {
	HourWatchTimes(ctx context.Context, userID, period string) ([]HourWatchTime, error)
	WeekdayWatchTimes(ctx context.Context, userID, period string) ([]WeekdayWatchTime, error)
	LongestSession(ctx context.Context, userID, period string) (LongestSession, bool, error)
	MostActiveDay(ctx context.Context, userID, period string) (MostActiveDay, bool, error)
}

func (client *Client) HourWatchTimes(ctx context.Context, userID, period string) ([]HourWatchTime, error) {
	var hours []HourWatchTime
	if err := client.getPersonalStats(ctx, "/EmbyInsights/PersonalStats/Hours", userID, period, &hours); err != nil {
		return nil, err
	}
	return hours, nil
}

func (client *Client) WeekdayWatchTimes(ctx context.Context, userID, period string) ([]WeekdayWatchTime, error) {
	var weekdays []WeekdayWatchTime
	if err := client.getPersonalStats(ctx, "/EmbyInsights/PersonalStats/Weekdays", userID, period, &weekdays); err != nil {
		return nil, err
	}
	return weekdays, nil
}

// LongestSession reports false when there were no playback sessions in the
// period (the plugin represents that as a zero-value DTO rather than an
// error, since an empty period is an expected outcome, not a failure).
func (client *Client) LongestSession(ctx context.Context, userID, period string) (LongestSession, bool, error) {
	var session LongestSession
	if err := client.getPersonalStats(ctx, "/EmbyInsights/PersonalStats/LongestSession", userID, period, &session); err != nil {
		return LongestSession{}, false, err
	}
	return session, session.ItemName != "", nil
}

func (client *Client) MostActiveDay(ctx context.Context, userID, period string) (MostActiveDay, bool, error) {
	var day MostActiveDay
	if err := client.getPersonalStats(ctx, "/EmbyInsights/PersonalStats/MostActiveDay", userID, period, &day); err != nil {
		return MostActiveDay{}, false, err
	}
	return day, day.WatchSeconds > 0, nil
}

func (client *Client) getPersonalStats(ctx context.Context, path, userID, period string, target any) error {
	query := url.Values{"UserId": {userID}, "Period": {period}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call Emby Insights %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Emby Insights %s returned %s", path, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode Emby Insights %s: %w", path, err)
	}
	return nil
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
