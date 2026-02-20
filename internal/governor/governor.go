package governor

import (
	log "log/slog"
	"strings"
	"time"

	"governor/pkg/proto"
)

type Governor struct {
	client   *proto.Client
	bootedAt time.Time
	schedule []Slot
	events   *eventStore
}

func New(client *proto.Client, schedulePath, eventsPath string) (*Governor, error) {
	events, err := newEventStore(eventsPath)
	if err != nil {
		return nil, err
	}
	g := &Governor{
		client:   client,
		bootedAt: time.Now(),
		events:   events,
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

// Cmd dispatches an incoming request by verb.
//
//	PING        -> PONG PONG
//	NEW  EVENT  -> OK EVENT <id>
//	STOP EVENT  -> OK EVENT <id> | ERR NAC
//	GET  UPTIME -> OK UPTIME <dur>
//	GET  SCHEDULE <weekday> -> OK SCHEDULE [<slot>...]
//	GET  EVENTS  -> OK EVENTS [<event>...]
//	GET  EVENT <id> -> OK EVENT <wire> | ERR NAC
func (g *Governor) Cmd(req *proto.Request) {
	msg := req.Msg
	log.Debug("CMD", "from", msg.From, "verb", msg.Verb, "noun", msg.Noun, "args", msg.Args)

	switch msg.Verb {
	case "OK", "ERR", "PONG":
		log.Debug("IGNORE", "verb", msg.Verb, "noun", msg.Noun, "from", msg.From)
		return
	case "PING":
		req.Reply("PONG", "PONG")
	case "NEW":
		g.cmdNew(req)
	case "STOP":
		g.cmdStop(req)
	case "GET":
		g.cmdGet(req)
	default:
		log.Warn("UNKNOWN VERB", "verb", msg.Verb, "from", msg.From)
		req.Reply("ERR", "VERB")
	}
}

func (g *Governor) cmdGet(req *proto.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "UPTIME":
		uptime := time.Since(g.bootedAt).Truncate(time.Second)
		log.Debug("GET UPTIME", "uptime", uptime, "from", msg.From)
		req.Reply("OK", "UPTIME", uptime.String())

	case "SCHEDULE":
		if len(msg.Args) < 1 {
			req.Reply("ERR", "ARGC")
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
		req.Reply("OK", "SCHEDULE", slots...)

	case "EVENTS":
		all := g.events.List()
		args := make([]string, len(all))
		for i := range all {
			args[i] = all[i].WireString()
		}
		log.Debug("GET EVENTS", "count", len(args), "from", msg.From)
		req.Reply("OK", "EVENTS", args...)

	case "EVENT":
		if len(msg.Args) < 1 {
			req.Reply("ERR", "ARGC")
			return
		}
		id := strings.TrimSpace(msg.Args[0])
		e, ok := g.events.Get(id)
		if !ok {
			req.Reply("ERR", "NAC")
			return
		}
		log.Debug("GET EVENT", "id", id, "from", msg.From)
		req.Reply("OK", "EVENT", e.WireString())

	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		req.Reply("ERR", "NOUN")
	}
}

func (g *Governor) cmdNew(req *proto.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "EVENT":
		if len(msg.Args) < 3 {
			req.Reply("ERR", "ARGC")
			return
		}
		title := strings.TrimSpace(msg.Args[0])
		if title == "" {
			log.Warn("NEW EVENT empty title", "from", msg.From)
			req.Reply("ERR", "TITLE")
			return
		}
		dateStr := msg.Args[1]
		timeStr := msg.Args[2]
		at, err := ParseEventAt(dateStr, timeStr)
		if err != nil {
			log.Warn("BAD EVENT TIME", "date", dateStr, "time", timeStr, "from", msg.From, "err", err)
			req.Reply("ERR", "TIME", dateStr, timeStr)
			return
		}
		var location, notes string
		if len(msg.Args) > 3 {
			location = strings.TrimSpace(msg.Args[3])
		}
		if len(msg.Args) > 4 {
			notes = strings.TrimSpace(msg.Args[4])
		}
		e := Event{Title: title, At: at, Location: location, Notes: notes}
		id, err := g.events.Add(e)
		if err != nil {
			log.Error("NEW EVENT add failed", "title", title, "from", msg.From, "err", err)
			req.Reply("ERR", "ADD", err.Error())
			return
		}
		log.Info("NEW EVENT", "id", id, "title", title, "at", at.Format("2006-01-02 15:04"), "from", msg.From)
		req.Reply("OK", "EVENT", id)
	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		req.Reply("ERR", "NOUN")
	}
}

func (g *Governor) cmdStop(req *proto.Request) {
	msg := req.Msg
	switch msg.Noun {
	case "EVENT":
		if len(msg.Args) < 1 {
			req.Reply("ERR", "ARGC")
			return
		}
		id := strings.TrimSpace(msg.Args[0])
		if !g.events.Delete(id) {
			log.Debug("STOP EVENT NOT FOUND", "id", id, "from", msg.From)
			req.Reply("ERR", "NAC")
			return
		}
		log.Info("STOP EVENT", "id", id, "from", msg.From)
		req.Reply("OK", "EVENT", id)
	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		req.Reply("ERR", "NOUN")
	}
}

func (g *Governor) Shutdown() {}
