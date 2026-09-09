package governor

import (
	"sort"
	"strings"
	"time"
)

// Agenda entry kinds, as they appear in the first sub-field of a wire
// entry.
const (
	AgendaClass = "CLASS"
	AgendaDue   = "DUE"
)

// agendaDateFmt is the colon-free date form used for the agenda's own
// date argument and inside DUE entries.
const agendaDateFmt = "2006.01.02"

// weekdayShort maps a Go weekday onto the three-letter form used in the
// schedule CSV.
var weekdayShort = [...]string{
	time.Sunday:    "Sun",
	time.Monday:    "Mon",
	time.Tuesday:   "Tue",
	time.Wednesday: "Wed",
	time.Thursday:  "Thu",
	time.Friday:    "Fri",
	time.Saturday:  "Sat",
}

// AgendaFor builds one day's agenda: the timetable slots for that
// weekday, in chronological order, followed by the deadlines that are
// visible on that day, soonest first.
//
// Entries are pre-rendered wire strings so the caller can hand them
// straight to reply(). Each is `<KIND>|<fields...>`:
//
//	CLASS|<start>|<end>|<title>|<location>
//	DUE|<title>|<YYYY.MM.DD>|<days remaining>
//
// The sub-separator is "|" and every field is passed through noColon, so
// nothing here can add a field to the frame. Days remaining is counted in
// whole calendar days from `day`, so an event later the same day reads 0.
func (g *Governor) AgendaFor(day time.Time) (date string, weekday string, entries []string) {
	day = day.Local()
	date = day.Format(agendaDateFmt)
	weekday = weekdayShort[day.Weekday()]

	// Timetable slots for this weekday.
	var classes []Slot
	for i := range g.schedule {
		if strings.EqualFold(g.schedule[i].Weekday, weekday) {
			classes = append(classes, g.schedule[i])
		}
	}
	sort.SliceStable(classes, func(i, j int) bool {
		return classes[i].Start < classes[j].Start
	})
	for _, s := range classes {
		entries = append(entries, strings.Join([]string{
			AgendaClass,
			noColon(s.Start), noColon(s.End),
			noColon(s.Title), noColon(s.Location),
		}, slotSep))
	}

	// Deadlines visible on this day, soonest first.
	midnight := startOfDay(day)
	var due []Event
	for _, e := range g.events.List() {
		if e.At.Before(midnight) {
			continue // already passed
		}
		if day.Before(e.DeadlineVisibleStart()) {
			continue // not yet in its visible window
		}
		due = append(due, e)
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].At.Before(due[j].At) })
	for _, e := range due {
		entries = append(entries, strings.Join([]string{
			AgendaDue,
			noColon(e.Title),
			e.At.Format(agendaDateFmt),
			itoa(daysBetween(midnight, e.At)),
		}, slotSep))
	}

	return date, weekday, entries
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// daysBetween counts whole calendar days from `from` to `to`, so an event
// later today is 0 and one tomorrow is 1 regardless of clock times.
func daysBetween(from, to time.Time) int {
	a := startOfDay(from)
	b := startOfDay(to)
	return int(b.Sub(a).Hours() / 24)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
