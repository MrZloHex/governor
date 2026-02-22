package governor

import (
	"fmt"
	"strings"
	"time"
)

// DefaultDeadlineVisibleDays is how many days before At an event starts appearing in GET:DEADLINES when VisibleFrom is not set.
const DefaultDeadlineVisibleDays = 7

type Event struct {
	ID          string     `json:"ID"`
	Title       string     `json:"Title"`
	At          time.Time  `json:"At"`
	Location    string     `json:"Location"`
	Notes       string     `json:"Notes"`
	VisibleFrom *time.Time `json:"VisibleFrom,omitempty"` // optional: date from which this event appears in GET:DEADLINES; nil = At - DefaultDeadlineVisibleDays
}

// eventWireFmt is colon-safe datetime for wire (no ":")
const eventWireFmt = "2006.01.02.15.04"

// Format: id|title|at|location|notes|visible_from (at e.g. 2025.02.21.14.30; visible_from YYYY.MM.DD or empty for default)
func (e Event) WireString() string {
	at := e.At.Format(eventWireFmt)
	visibleFrom := ""
	if e.VisibleFrom != nil {
		visibleFrom = e.VisibleFrom.Format("2006.01.02")
	}
	return strings.Join([]string{
		noColon(e.ID), noColon(e.Title), at,
		noColon(e.Location), noColon(e.Notes), visibleFrom,
	}, slotSep)
}

// DeadlineVisibleStart returns the time from which this event appears in GET:DEADLINES.
func (e Event) DeadlineVisibleStart() time.Time {
	if e.VisibleFrom != nil {
		return *e.VisibleFrom
	}
	return e.At.AddDate(0, 0, -DefaultDeadlineVisibleDays)
}

// ParseEventAt parses date (YYYY.MM.DD) and time (HH.MM or HH.MM.SS) in local time
func ParseEventAt(dateStr, timeStr string) (time.Time, error) {
	var y, mo, d, h, min, sec int
	_, err := fmt.Sscanf(strings.TrimSpace(dateStr), "%d.%d.%d", &y, &mo, &d)
	if err != nil {
		return time.Time{}, fmt.Errorf("date: %w", err)
	}
	if mo < 1 || mo > 12 {
		return time.Time{}, fmt.Errorf("month must be 1–12, got %d", mo)
	}
	if d < 1 || d > 31 {
		return time.Time{}, fmt.Errorf("day must be 1–31, got %d", d)
	}

	n, _ := fmt.Sscanf(strings.TrimSpace(timeStr), "%d.%d.%d", &h, &min, &sec)
	if n < 2 {
		return time.Time{}, fmt.Errorf("time: need at least HH.MM")
	}
	if h < 0 || h > 23 {
		return time.Time{}, fmt.Errorf("hour must be 0–23, got %d", h)
	}
	if min < 0 || min > 59 {
		return time.Time{}, fmt.Errorf("minute must be 0–59, got %d", min)
	}
	if sec < 0 || sec > 59 {
		return time.Time{}, fmt.Errorf("second must be 0–59, got %d", sec)
	}

	t := time.Date(y, time.Month(mo), d, h, min, sec, 0, time.Local)

	// time.Date normalizes (e.g. Feb 30 -> Mar 2); check we didn't roll over.
	if t.Day() != d || t.Month() != time.Month(mo) || t.Year() != y {
		return time.Time{}, fmt.Errorf("invalid date: %04d.%02d.%02d (e.g. no Feb 30)", y, mo, d)
	}
	return t, nil
}

// ParseVisibleFromDate parses an optional "visible from" date (YYYY.MM.DD) for deadline visibility.
// Returns nil if s is empty or invalid (caller can use default).
func ParseVisibleFromDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var y, mo, d int
	_, err := fmt.Sscanf(s, "%d.%d.%d", &y, &mo, &d)
	if err != nil {
		return nil, fmt.Errorf("visible_from date: %w", err)
	}
	if mo < 1 || mo > 12 {
		return nil, fmt.Errorf("month must be 1–12, got %d", mo)
	}
	if d < 1 || d > 31 {
		return nil, fmt.Errorf("day must be 1–31, got %d", d)
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	if t.Day() != d || t.Month() != time.Month(mo) || t.Year() != y {
		return nil, fmt.Errorf("invalid date: %04d.%02d.%02d", y, mo, d)
	}
	return &t, nil
}
