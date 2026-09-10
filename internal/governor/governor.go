package governor

import (
	log "log/slog"
	"sort"
	"strings"
	"time"

	"github.com/MrZloHex/monolink"
)

const DefaultDeadlinePeriod = 7 * 24 * time.Hour

type Governor struct {
	client         *monolink.Client
	bootedAt       time.Time
	schedule       []Slot
	events         *eventStore
	deadlinePeriod time.Duration
}

func New(client *monolink.Client, schedulePath, eventsPath string) (*Governor, error) {
	events, err := newEventStore(eventsPath)
	if err != nil {
		return nil, err
	}

	g := &Governor{
		client:         client,
		bootedAt:       time.Now(),
		events:         events,
		deadlinePeriod: DefaultDeadlinePeriod,
	}

	if schedulePath != "" {
		slots, err := LoadScheduleFromCSV(schedulePath)
		if err != nil {
			return nil, err
		}
		g.schedule = slots
		log.Debug("schedule loaded", "path", schedulePath, "slots", len(slots))
	}

	return g, nil
}

func (g *Governor) reply(req *monolink.Request, verb, noun string, args ...string) {
	if err := req.Reply(verb, noun, args...); err != nil {
		log.Warn("reply failed", "to", req.Msg.From, "verb", verb, "noun", noun, "err", err)
	}
}

// Cmd dispatches an incoming request by verb.
//
//	PING        -> PONG PONG (v1) | OK PING (v2)
//	NEW  EVENT  -> OK EVENT <id>
//	STOP EVENT  -> OK EVENT <id> | ERR NAC
//	GET  AGENDA [<date>]    -> OK AGENDA <date> <weekday> [<entry>...]
//	GET  UPTIME -> OK UPTIME <dur>
//	GET  SCHEDULE <weekday> -> OK SCHEDULE [<slot>...]
//	GET  EVENTS     -> OK EVENTS [<event>...]
//	GET  EVENT <id> -> OK EVENT <wire> | ERR NAC
//	GET  DEADLINES [day|week|month] -> OK DEADLINES [<event>...]  (no arg: configured period; else calendar window)
func (g *Governor) Cmd(req *monolink.Request) {
	msg := req.Msg
	log.Debug("CMD", "from", msg.From, "verb", msg.Verb, "noun", msg.Noun, "args", msg.Args)

	switch msg.Verb {
	case "OK", "ERR", "PONG":
		log.Debug("IGNORE", "verb", msg.Verb, "noun", msg.Noun, "from", msg.From)
		return
	case "PING":
		// v2 answers OK:PING, correlated by id like any reply (SPEC §43).
		if msg.Version == monolink.V2 {
			g.reply(req, "OK", "PING")
		} else {
			g.reply(req, "PONG", "PONG")
		}
	case "NEW":
		g.cmdNew(req)
	case "STOP":
		g.cmdStop(req)
	case "GET":
		g.cmdGet(req)
	default:
		log.Warn("UNKNOWN VERB", "verb", msg.Verb, "from", msg.From)
		g.reply(req, "ERR", "VERB")
	}
}

func (g *Governor) cmdGet(req *monolink.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "UPTIME":
		uptime := time.Since(g.bootedAt).Truncate(time.Second)
		log.Debug("GET UPTIME", "uptime", uptime, "from", msg.From)
		g.reply(req, "OK", "UPTIME", uptime.String())

	case "SCHEDULE":
		if len(msg.Args) < 1 {
			g.reply(req, "ERR", "ARGC")
			return
		}
		weekday := strings.TrimSpace(msg.Args[0])
		var slots []string
		for i := range g.schedule {
			if strings.EqualFold(g.schedule[i].Weekday, weekday) {
				slots = append(slots, g.schedule[i].WireString())
			}
		}
		log.Debug("GET SCHEDULE", "weekday", weekday, "slots", len(slots), "from", msg.From)
		g.reply(req, "OK", "SCHEDULE", slots...)

	case "EVENTS":
		all := g.events.List()
		args := make([]string, len(all))
		for i := range all {
			args[i] = all[i].WireString()
		}
		log.Debug("GET EVENTS", "count", len(args), "from", msg.From)
		g.reply(req, "OK", "EVENTS", args...)

	case "EVENT":
		if len(msg.Args) < 1 {
			g.reply(req, "ERR", "ARGC")
			return
		}
		id := strings.TrimSpace(msg.Args[0])
		e, ok := g.events.Get(id)
		if !ok {
			g.reply(req, "ERR", "NAC")
			return
		}
		log.Debug("GET EVENT", "id", id, "from", msg.From)
		g.reply(req, "OK", "EVENT", e.WireString())

	case "AGENDA":
		// GET:AGENDA[:<YYYY.MM.DD>] -- everything one day needs, in a
		// single reply. Composed here rather than on the client because
		// the intended caller is an ESP8266, where two round trips and
		// this much string handling are expensive.
		day := time.Now()
		if len(msg.Args) >= 1 && strings.TrimSpace(msg.Args[0]) != "" {
			d, err := time.ParseInLocation(agendaDateFmt, strings.TrimSpace(msg.Args[0]), time.Local)
			if err != nil {
				log.Warn("GET AGENDA bad date", "raw", msg.Args[0], "from", msg.From)
				g.reply(req, "ERR", "DATE")
				return
			}
			day = d
		}
		date, weekday, entries := g.AgendaFor(day)
		args := append([]string{date, weekday}, entries...)
		log.Debug("GET AGENDA", "date", date, "weekday", weekday, "entries", len(entries), "from", msg.From)
		g.reply(req, "OK", "AGENDA", args...)

	case "DEADLINES":
		all := g.events.List()
		now := time.Now()
		// Soonest first: the store keeps insertion order, which is not a
		// useful order to read a deadline list in.
		sort.SliceStable(all, func(i, j int) bool { return all[i].At.Before(all[j].At) })
		var args []string

		if len(msg.Args) >= 1 {
			// Specific calendar window: GET:DEADLINES:DAY|WEEK|MONTH|YEAR
			// Include event if At is in [start,end] and now is within event's visible window [visibleStart, At]
			start, end := periodBounds(msg.Args[0])
			if start.IsZero() && end.IsZero() {
				log.Warn("GET DEADLINES unknown period", "period", msg.Args[0], "from", msg.From)
				g.reply(req, "ERR", "PERIOD")
				return
			}
			// An explicit window means "show me what falls in it", so the
			// visible-from rule does not apply here -- that rule exists to
			// keep the no-argument query to what is currently on the radar.
			// Applying both meant GET:DEADLINES:month returned nothing at
			// all when every event was more than the default seven days
			// out, which is exactly when you would ask.
			for i := range all {
				e := &all[i]
				at := e.At
				if !at.Before(start) && !at.After(end) && !now.After(at) {
					args = append(args, e.WireString())
				}
			}

			log.Debug("GET DEADLINES", "window", msg.Args[0], "start", start.Format("2006-01-02"), "end", end.Format("2006-01-02"), "count", len(args), "from", msg.From)
		} else {
			// No window arg: return events that are currently in their visible window (visibleStart <= now <= At).
			// Default visibleStart = At - 7 days; can be overridden per event via VisibleFrom.
			for i := range all {
				e := &all[i]
				visibleStart := e.DeadlineVisibleStart()
				if !now.Before(visibleStart) && !now.After(e.At) {
					args = append(args, e.WireString())
				}
			}
			log.Debug("GET DEADLINES", "now", now.Format("2006-01-02 15:04"), "count", len(args), "from", msg.From)
		}
		g.reply(req, "OK", "DEADLINES", args...)

	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		g.reply(req, "ERR", "NOUN")
	}
}

func (g *Governor) cmdNew(req *monolink.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "EVENT":
		if len(msg.Args) < 3 {
			g.reply(req, "ERR", "ARGC")
			return
		}
		title := strings.TrimSpace(msg.Args[0])
		if title == "" {
			log.Warn("NEW EVENT empty title", "from", msg.From)
			g.reply(req, "ERR", "TITLE")
			return
		}
		dateStr := msg.Args[1]
		timeStr := msg.Args[2]
		at, err := ParseEventAt(dateStr, timeStr)
		if err != nil {
			log.Warn("BAD EVENT TIME", "date", dateStr, "time", timeStr, "from", msg.From, "err", err)
			g.reply(req, "ERR", "TIME", dateStr, timeStr)
			return
		}
		var location, notes string
		if len(msg.Args) > 3 {
			location = strings.TrimSpace(msg.Args[3])
		}
		if len(msg.Args) > 4 {
			notes = strings.TrimSpace(msg.Args[4])
		}
		var visibleFrom *time.Time
		if len(msg.Args) > 5 {
			vf, err := ParseVisibleFromDate(msg.Args[5])
			if err != nil {
				log.Warn("NEW EVENT bad visible_from", "visible_from", msg.Args[5], "from", msg.From, "err", err)
				g.reply(req, "ERR", "VISIBLE", msg.Args[5])
				return
			}
			visibleFrom = vf
		}
		e := Event{Title: title, At: at, Location: location, Notes: notes, VisibleFrom: visibleFrom}
		id, err := g.events.Add(e)
		if err != nil {
			log.Error("NEW EVENT add failed", "title", title, "from", msg.From, "err", err)
			g.reply(req, "ERR", "ADD", err.Error())
			return
		}
		log.Info("NEW EVENT", "id", id, "title", title, "at", at.Format("2006-01-02 15:04"), "from", msg.From)
		g.reply(req, "OK", "EVENT", id)
	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		g.reply(req, "ERR", "NOUN")
	}
}

func (g *Governor) cmdStop(req *monolink.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "EVENT":
		if len(msg.Args) < 1 {
			g.reply(req, "ERR", "ARGC")
			return
		}
		id := strings.TrimSpace(msg.Args[0])
		if !g.events.Delete(id) {
			log.Debug("STOP EVENT NOT FOUND", "id", id, "from", msg.From)
			g.reply(req, "ERR", "NAC")
			return
		}
		log.Info("STOP EVENT", "id", id, "from", msg.From)
		g.reply(req, "OK", "EVENT", id)
	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		g.reply(req, "ERR", "NOUN")
	}
}

func (g *Governor) Shutdown() {}
