package governor

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type Slot struct {
	Weekday  string // Mon, Tue, ...
	Start    string // 10:45
	End      string // 12:10
	Title    string
	Location string
	Tags     string
}

const slotSep = "|"

func noColon(s string) string { return strings.ReplaceAll(s, ":", ".") }

// Format: Weekday|Start|End|Title|Location|Tags
func (s Slot) WireString() string {
	return strings.Join([]string{
		noColon(s.Weekday), noColon(s.Start), noColon(s.End),
		noColon(s.Title), noColon(s.Location), noColon(s.Tags),
	}, slotSep)
}

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
