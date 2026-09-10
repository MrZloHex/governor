package governor

import (
	"strings"
	"testing"
	"time"
)

func testGovernor(t *testing.T, slots []Slot, events []Event) *Governor {
	t.Helper()
	store, err := newEventStore("")
	if err != nil {
		t.Fatalf("newEventStore: %v", err)
	}
	for _, e := range events {
		if _, err := store.Add(e); err != nil {
			t.Fatalf("seed event %s: %v", e.ID, err)
		}
	}
	return &Governor{schedule: slots, events: store, deadlinePeriod: DefaultDeadlinePeriod}
}

func TestAgendaOrdersClassesByStart(t *testing.T) {
	// Thursday 2026-09-10.
	day := time.Date(2026, 9, 10, 9, 0, 0, 0, time.Local)

	g := testGovernor(t, []Slot{
		{Weekday: "Thu", Start: "13:55", End: "15:20", Title: "Late", Location: "B"},
		{Weekday: "Thu", Start: "09:00", End: "10:25", Title: "Early", Location: "A"},
		{Weekday: "Fri", Start: "08:00", End: "09:00", Title: "OtherDay", Location: "C"},
	}, nil)

	date, weekday, entries := g.AgendaFor(day)
	if date != "2026.09.10" {
		t.Errorf("date = %q, want 2026.09.10", date)
	}
	if weekday != "Thu" {
		t.Errorf("weekday = %q, want Thu", weekday)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries (other weekdays excluded), got %d: %v", len(entries), entries)
	}
	if !strings.HasPrefix(entries[0], "CLASS|09.00|10.25|Early|A") {
		t.Errorf("first entry not the earliest class: %q", entries[0])
	}
	if !strings.HasPrefix(entries[1], "CLASS|13.55|15.20|Late|B") {
		t.Errorf("second entry wrong: %q", entries[1])
	}
}

func TestAgendaTimesAreColonFree(t *testing.T) {
	// The CSV holds "09:00"; a colon reaching the wire would split the
	// frame and push FROM out of position.
	day := time.Date(2026, 9, 10, 9, 0, 0, 0, time.Local)
	g := testGovernor(t, []Slot{
		{Weekday: "Thu", Start: "09:00", End: "10:25", Title: "A:B", Location: "Room:1"},
	}, nil)

	_, _, entries := g.AgendaFor(day)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if strings.Contains(entries[0], ":") {
		t.Fatalf("entry contains a colon: %q", entries[0])
	}
}

func TestAgendaDeadlines(t *testing.T) {
	day := time.Date(2026, 9, 10, 9, 0, 0, 0, time.Local)
	at := func(y, m, d int) time.Time {
		return time.Date(y, time.Month(m), d, 12, 0, 0, 0, time.Local)
	}
	visible := func(y, m, d int) *time.Time {
		v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
		return &v
	}

	g := testGovernor(t, nil, []Event{
		{ID: "soon", Title: "Soon", At: at(2026, 9, 12)},   // 2 days out, inside default 7d window
		{ID: "today", Title: "Today", At: at(2026, 9, 10)}, // later today
		{ID: "far", Title: "Far", At: at(2026, 12, 1)},     // outside default window
		{ID: "past", Title: "Past", At: at(2026, 9, 1)},    // already gone
		{ID: "early", Title: "Early", At: at(2026, 10, 20), // far out, but explicitly visible
			VisibleFrom: visible(2026, 9, 1)},
	})

	_, _, entries := g.AgendaFor(day)

	var titles []string
	for _, e := range entries {
		f := strings.Split(e, "|")
		if f[0] != AgendaDue {
			t.Fatalf("unexpected entry kind in %q", e)
		}
		titles = append(titles, f[1])
	}

	want := []string{"Today", "Soon", "Early"} // soonest first
	if len(titles) != len(want) {
		t.Fatalf("got %v, want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("got %v, want %v", titles, want)
		}
	}

	// Days remaining is whole calendar days, so "later today" is 0.
	f := strings.Split(entries[0], "|")
	if f[3] != "0" {
		t.Errorf("an event later today should be 0 days out, got %q", f[3])
	}
	f = strings.Split(entries[1], "|")
	if f[3] != "2" {
		t.Errorf("an event in 2 days should be 2, got %q", f[3])
	}
	if f[2] != "2026.09.12" {
		t.Errorf("due date = %q, want 2026.09.12", f[2])
	}
}

func TestAgendaEmptyDay(t *testing.T) {
	g := testGovernor(t, nil, nil)
	date, weekday, entries := g.AgendaFor(time.Date(2026, 9, 13, 9, 0, 0, 0, time.Local))
	if len(entries) != 0 {
		t.Fatalf("want no entries, got %v", entries)
	}
	if date == "" || weekday != "Sun" {
		t.Fatalf("date/weekday still expected on an empty day: %q %q", date, weekday)
	}
}

func TestDaysBetweenIgnoresClockTime(t *testing.T) {
	// 23:59 today to 00:01 tomorrow is one calendar day, not zero.
	from := time.Date(2026, 9, 10, 23, 59, 0, 0, time.Local)
	to := time.Date(2026, 9, 11, 0, 1, 0, 0, time.Local)
	if got := daysBetween(from, to); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

func TestPeriodBoundsYearIsCalendarYear(t *testing.T) {
	start, end := periodBounds("year")
	if start.Month() != time.January || start.Day() != 1 {
		t.Errorf("year should start on 1 January, got %v", start)
	}
	if end.Year() != start.Year() || end.Month() != time.December {
		t.Errorf("year should end in December of the same year, got %v", end)
	}
}

func TestPeriodBoundsDayAndMonth(t *testing.T) {
	s, e := periodBounds("day")
	if e.Sub(s) > 24*time.Hour {
		t.Errorf("day window is %v", e.Sub(s))
	}
	s, e = periodBounds("month")
	if s.Day() != 1 || e.Month() != s.Month() {
		t.Errorf("month window %v .. %v", s, e)
	}
	if s, e := periodBounds("banana"); !s.IsZero() || !e.IsZero() {
		t.Error("an unknown period should report zero bounds")
	}
}
