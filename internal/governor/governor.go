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
}

func New(client *proto.Client, schedulePath string) (*Governor, error) {
	g := &Governor{
		client:   client,
		bootedAt: time.Now(),
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
//	PING PING    -> PONG PONG
//	GET  UPTIME  -> OK UPTIME <dur>
func (g *Governor) Cmd(req *proto.Request) {
	msg := req.Msg
	log.Debug("CMD", "from", msg.From, "verb", msg.Verb, "noun", msg.Noun, "args", msg.Args)

	switch msg.Verb {
	case "OK", "ERR", "PONG":
		log.Debug("IGNORE", "verb", msg.Verb, "noun", msg.Noun, "from", msg.From)
		return
	case "PING":
		req.Reply("PONG", "PONG")
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

	default:
		log.Warn("UNKNOWN NOUN", "noun", msg.Noun, "from", msg.From)
		req.Reply("ERR", "NOUN")
	}
}

func (g *Governor) Shutdown() {}
