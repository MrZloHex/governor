package governor

import (
	"strings"
	"time"
)

func periodBounds(period string) (start, end time.Time) {
	now := time.Now().Local()
	period = strings.TrimSpace(period)
	switch strings.ToLower(period) {
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		end = start.Add(24*time.Hour - time.Nanosecond)
	case "week":
		weekday := now.Weekday()
		daysSinceMonday := int(weekday) - 1
		if daysSinceMonday < 0 {
			daysSinceMonday += 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -daysSinceMonday)
		end = start.AddDate(0, 0, 7).Add(-time.Nanosecond)
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "year":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(1, 0, 0).Add(-time.Nanosecond)
	}
	return start, end
}
