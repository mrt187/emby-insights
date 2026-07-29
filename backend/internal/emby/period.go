package emby

import (
	"fmt"
	"time"
)

// periodRange mirrors PersonalStatisticsPeriods.Current in the Emby plugin
// (Services/PersonalWatchTimeReader.cs), so "this week/month/year" means the
// same thing everywhere in the product.
func periodRange(period string, now time.Time) (time.Time, time.Time, error) {
	utcNow := now.UTC()
	dayStart := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)

	var from time.Time
	switch period {
	case "week":
		offset := (int(dayStart.Weekday()) + 6) % 7
		from = dayStart.AddDate(0, 0, -offset)
	case "month":
		from = time.Date(dayStart.Year(), dayStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "year":
		from = time.Date(dayStart.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown period %q", period)
	}
	return from, utcNow, nil
}
