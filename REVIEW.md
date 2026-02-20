# Governor — Project Review

## Overview

**Governor** is a concentrator node for schedule and events: static weekly CSV schedule, persistent events (add/list/get/remove), uptime, and ping/pong. Protocol is colon-separated (`TO:VERB:NOUN[:ARGS...]:FROM`), matching the achtung-style system.

---

## Structure

| Path | Purpose |
|------|--------|
| `cmd/governor/main.go` | Flags, logger, proto client, governor init, single `*` handler, connect, signal wait, shutdown |
| `internal/governor/governor.go` | Governor struct, `Cmd()` dispatch (PING, NEW, STOP, GET), cmdNew/cmdStop/cmdGet |
| `internal/governor/schedule.go` | Slot type, WireString (colon-safe), LoadScheduleFromCSV |
| `internal/governor/event.go` | Event type, WireString, ParseEventAt (date + time) |
| `internal/governor/eventstore.go` | In-memory store + JSON load/save, Add/List/Get/Delete |
| `pkg/proto/message.go` | Message, Parse, Encode, Request.Reply |
| `pkg/proto/client.go` | Client, Connect, Handle, reconnect, dispatch |

Layout is clear: `cmd` for entrypoint, `internal/governor` for node logic, `pkg/proto` shared with other nodes.

---

## What Works Well

- **Protocol consistency**: Same verb/noun and `ERR`/`OK` style as achtung; colon-safe wire encoding (pipe + dot) for schedule and events.
- **Persistence**: Events loaded on startup, saved after every Add/Delete; missing file is OK; corrupt JSON fails startup with error.
- **ID generation**: `ev1`, `ev2`, … with `nextID` restored from max loaded id so no clashes after reboot.
- **Concurrency**: eventStore uses RWMutex; handlers run in goroutines (proto dispatch); Save() only holds RLock while building slice.
- **Graceful shutdown**: SIGINT/SIGTERM → Shutdown() + Close(); no leaked goroutines.
- **Docs**: README matches achtung style and documents protocol and flags.

---

## Issues & Fixes

### 1. Misleading error on startup (fixed)

`governor.New()` can fail from **either** schedule load **or** events load, but main logs `"Failed to load schedule"` in both cases. That was updated to a generic message so events load failure is not misreported.

### 2. Cmd doc comment outdated (fixed)

The `Cmd` doc comment only mentioned PING and GET UPTIME. It was updated to list all verbs/nouns (NEW, STOP, GET SCHEDULE/EVENTS/EVENT).

### 3. Empty event title allowed — **fixed**

`NEW:EVENT` now rejects empty title (after trim): reply `ERR:TITLE`, log `NEW EVENT empty title`, and do not add.

### 4. Event in memory when Save() fails — **fixed**

On Add, if `Save()` fails we remove the event from the map, return error, log `events save failed after add`, and reply `ERR:ADD`. On Delete, if `Save()` fails we log `events save failed after delete` (in-memory state already updated; no undo).

### 5. ParseEventAt has no range checks

`ParseEventAt(date, time)` does not validate month 1–12, day 1–31, hour 0–23, minute 0–59. Invalid numbers can produce odd `time.Time` values. Low risk for controlled input; adding validation would make errors clearer.

### 6. Event list order

`eventstore.List()` returns map iteration order (non-deterministic). README does not promise order. If clients expect “by time”, consider sorting by `At` before building the reply (or document “unordered”).

### 7. Duplicate IDs in events.json

If the JSON file has two events with the same id, Load() overwrites; last one wins. Rare in practice; could dedupe or document.

---

## Proto Package

- **message.go**: Parse splits on `:`, so any colon inside an arg would break parsing; governor avoids that with WireString (pipe + dot).
- **client.go**: Handlers run in goroutines; reconnect and inbox behavior are clear. No issues found.

---

## Recommendations

1. **Done**: Fix main error message and Cmd doc comment (see below).
2. **Optional**: Reject empty title in NEW:EVENT.
3. **Optional**: In eventstore.Add, on Save() failure remove the event from the map and return error for atomic behavior.
4. **Optional**: Sort events by At in GET:EVENTS (or document order).
5. **Optional**: Validate date/time ranges in ParseEventAt and return a clear error.
6. **Tests**: Add unit tests for ParseEventAt, WireString (slot + event), eventstore Load/Save/nextID, and optionally CSV parsing.

---

## Summary

The project is in good shape: clear layout, consistent protocol, safe wire encoding, and persistent events with correct ID handling. The only concrete fixes applied were the startup error message and the Cmd doc comment; the rest are optional hardening and documentation improvements.
