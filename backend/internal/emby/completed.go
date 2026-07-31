package emby

import (
	"context"
	"sort"
	"time"
)

// completedItemsQueryLimit is higher than watchedItemsLimit because a period
// (especially "year") can plausibly contain more than 24 completed titles;
// unlike the all-time watched lists, this result is not capped afterwards.
const completedItemsQueryLimit = 200

type CompletedReader interface {
	CompletedMovies(ctx context.Context, userID string, libraryIDs []string, from, to time.Time) ([]WatchedItem, error)
	CompletedSeries(ctx context.Context, userID string, libraryIDs []string, from, to time.Time) ([]WatchedItem, error)
}

// CompletedMovies reads movies played within [from, to), scoped to the given
// library IDs — the same period boundaries used for /api/stats.
func (client *Client) CompletedMovies(ctx context.Context, userID string, libraryIDs []string, from, to time.Time) ([]WatchedItem, error) {
	return client.completedItems(ctx, userID, "Movie", libraryIDs, from, to)
}

// CompletedSeries reads series played within [from, to), scoped to the given
// library IDs.
func (client *Client) CompletedSeries(ctx context.Context, userID string, libraryIDs []string, from, to time.Time) ([]WatchedItem, error) {
	return client.completedItems(ctx, userID, "Series", libraryIDs, from, to)
}

func (client *Client) completedItems(ctx context.Context, userID, itemType string, libraryIDs []string, from, to time.Time) ([]WatchedItem, error) {
	if len(libraryIDs) == 0 {
		return nil, nil
	}

	var candidates []embyWatchedCandidate
	for _, libraryID := range libraryIDs {
		found, err := client.watchedItemsInLibrary(ctx, userID, itemType, libraryID, completedItemsQueryLimit)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, found...)
	}

	items := make([]WatchedItem, 0, len(candidates))
	for _, item := range candidates {
		lastPlayedDate, err := client.lastPlayedDateFor(ctx, userID, itemType, item.Id)
		if err != nil {
			return nil, err
		}
		// Emby's own LastPlayedDate carries fractional seconds (e.g.
		// "2026-07-02T07:20:51.0000000Z"), which time.RFC3339 rejects.
		played, err := time.Parse(time.RFC3339Nano, lastPlayedDate)
		if err != nil || played.Before(from) || !played.Before(to) {
			continue
		}

		var posterURL string
		if item.ImageTags.Primary != "" {
			posterURL = ImageURL(item.Id, "Primary", item.ImageTags.Primary, 400)
		}
		items = append(items, WatchedItem{ID: item.Id, Title: item.Name, PosterURL: posterURL, Genres: item.Genres, LastPlayedDate: lastPlayedDate})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].LastPlayedDate > items[j].LastPlayedDate })
	return items, nil
}

// PeriodBounds mirrors the Emby plugin's own period calculation
// (PersonalStatisticsPeriods.Current in PersonalWatchTimeReader.cs) exactly,
// so the completed-items lists line up with the /api/stats counts for the
// same period: week starts Monday 00:00 UTC, month/year start on the 1st,
// both ending at "now".
func PeriodBounds(period string, now time.Time) (time.Time, time.Time) {
	utcNow := now.UTC()
	dayStart := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)

	var from time.Time
	switch period {
	case "week":
		offset := (int(dayStart.Weekday()) + 6) % 7
		from = dayStart.AddDate(0, 0, -offset)
	case "month":
		from = time.Date(dayStart.Year(), dayStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	default: // "year"
		from = time.Date(dayStart.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return from, utcNow
}
