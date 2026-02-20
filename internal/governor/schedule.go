package governor

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// Slot is one weekly schedule entry (weekday + start/end time, title, location, tags).
type Slot struct {
	Weekday  string // Mon, Tue, ...
	Start    string // 10:45
	End      string // 12:10
	Title    string
	Location string
	Tags     string
}

// slotSep is used to join slot fields in wire format (avoids colon in protocol).
const slotSep = "|"

// noColon replaces colons so the string is safe in protocol (wire uses ":" as separator).
func noColon(s string) string { return strings.ReplaceAll(s, ":", ".") }

// WireString encodes the slot for protocol reply. All fields are colon-safe so the
// message can be split on ":". Format: Weekday|Start|End|Title|Location|Tags
// (times e.g. 10.45 instead of 10:45).
func (s Slot) WireString() string {
	return strings.Join([]string{
		noColon(s.Weekday), noColon(s.Start), noColon(s.End),
		noColon(s.Title), noColon(s.Location), noColon(s.Tags),
	}, slotSep)
}

// LoadScheduleFromCSV reads a weekly schedule from a CSV file.
// CSV header: weekday,start,end,title,location,tags
// Empty rows and rows with empty title are skipped.
func LoadScheduleFromCSV(path string) ([]Slot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open schedule: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // allow variable number of fields
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	if len(rows) < 2 {
		return nil, nil
	}

	// skip header
	var slots []Slot
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		weekday := strings.TrimSpace(row[0])
		start := strings.TrimSpace(row[1])
		end := strings.TrimSpace(row[2])
		title := strings.TrimSpace(row[3])
		location := strings.TrimSpace(row[4])
		tags := strings.TrimSpace(row[5])
		if title == "" {
			continue
		}
		slots = append(slots, Slot{
			Weekday:  weekday,
			Start:    start,
			End:      end,
			Title:    title,
			Location: location,
			Tags:     tags,
		})
	}
	return slots, nil
}
