package governor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MrZloHex/monolink"
)

// NEXT.DEADLINE is the soonest event still ahead: past ones do not count,
// and it empties when nothing is ahead.
func TestNextDeadlineIsTheSoonestAhead(t *testing.T) {
	store, err := newEventStore(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	g := &Governor{client: monolink.New("GOVERNOR", "ws://127.0.0.1:1"), events: store}
	g.register()
	if got := g.nextDeadline.Get(); got != "" {
		t.Fatalf("no events: NEXT.DEADLINE = %q", got)
	}

	soon := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	for _, e := range []Event{
		{Title: "passed", At: time.Now().Add(-time.Hour)},
		{Title: "later", At: soon.Add(24 * time.Hour)},
		{Title: "soon", At: soon},
	} {
		if _, err := store.Add(e); err != nil {
			t.Fatal(err)
		}
	}
	g.announce()
	if got := g.nextDeadline.Get(); got != soon.Format(time.RFC3339) {
		t.Fatalf("NEXT.DEADLINE = %q, want %q", got, soon.Format(time.RFC3339))
	}
}
