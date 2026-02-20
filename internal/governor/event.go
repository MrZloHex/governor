package governor

import (
	"fmt"
	"strings"
	"time"
)

// Event is a one-off event (title, datetime, optional location/notes).
type Event struct {
	ID       string
	Title    string
	At       time.Time
	Location string
	Notes    string
}

// eventWireFmt is colon-safe datetime for wire (no ":").
const eventWireFmt = "2006.01.02.15.04"

// WireString encodes the event for protocol reply (colon-safe).
// Format: id|title|at|location|notes (at e.g. 2025.02.21.14.30).
func (e Event) WireString() string {
	at := e.At.Format(eventWireFmt)
	return strings.Join([]string{
		noColon(e.ID), noColon(e.Title), at,
		noColon(e.Location), noColon(e.Notes),
	}, slotSep)
}

// ParseEventAt parses date (YYYY.MM.DD) and time (HH.MM or HH.MM.SS) in local time.
func ParseEventAt(dateStr, timeStr string) (time.Time, error) {
	var y, mo, d, h, min, sec int
	_, err := fmt.Sscanf(strings.TrimSpace(dateStr), "%d.%d.%d", &y, &mo, &d)
	if err != nil {
		return time.Time{}, fmt.Errorf("date: %w", err)
	}
	n, _ := fmt.Sscanf(strings.TrimSpace(timeStr), "%d.%d.%d", &h, &min, &sec)
	if n < 2 {
		return time.Time{}, fmt.Errorf("time: need at least HH.MM")
	}
	return time.Date(y, time.Month(mo), d, h, min, sec, 0, time.Local), nil
}
