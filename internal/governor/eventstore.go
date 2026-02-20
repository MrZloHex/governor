package governor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
)

type eventStore struct {
	mu     sync.RWMutex
	byID   map[string]*Event
	nextID int
	path   string
}

func newEventStore(path string) (*eventStore, error) {
	s := &eventStore{byID: make(map[string]*Event), path: path}
	if path != "" {
		if err := s.Load(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *eventStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var list []Event
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for i := range list {
		e := &list[i]
		s.byID[e.ID] = e
		if n := parseEventID(e.ID); n >= s.nextID {
			s.nextID = n + 1
		}
	}
	return nil
}

func parseEventID(id string) int {
	const prefix = "ev"
	if strings.HasPrefix(id, prefix) {
		n, _ := strconv.Atoi(id[len(prefix):])
		return n
	}
	return 0
}

func (s *eventStore) Save() error {
	if s.path == "" {
		return nil
	}
	s.mu.RLock()
	list := make([]Event, 0, len(s.byID))
	for _, e := range s.byID {
		list = append(list, *e)
	}
	s.mu.RUnlock()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *eventStore) Add(e Event) (string, error) {
	s.mu.Lock()
	s.nextID++
	id := fmt.Sprintf("ev%d", s.nextID)
	e.ID = id
	cp := e
	s.byID[id] = &cp
	s.mu.Unlock()
	if err := s.Save(); err != nil {
		slog.Error("events save failed after add", "path", s.path, "id", id, "err", err)
		s.mu.Lock()
		delete(s.byID, id)
		s.mu.Unlock()
		return "", err
	}
	return id, nil
}

func (s *eventStore) List() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, 0, len(s.byID))
	for _, e := range s.byID {
		out = append(out, *e)
	}
	return out
}

func (s *eventStore) Get(id string) (Event, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.byID[id]
	if !ok {
		return Event{}, false
	}
	return *e, true
}

func (s *eventStore) Delete(id string) bool {
	s.mu.Lock()
	if _, ok := s.byID[id]; !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.byID, id)
	s.mu.Unlock()
	if err := s.Save(); err != nil {
		slog.Error("events save failed after delete", "path", s.path, "id", id, "err", err)
	}
	return true
}
