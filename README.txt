███╗   ███╗ ██████╗ ███╗   ██╗ ██████╗ ██╗     ██╗████████╗██╗  ██╗
████╗ ████║██╔═══██╗████╗  ██║██╔═══██╗██║     ██║╚══██╔══╝██║  ██║
██╔████╔██║██║   ██║██╔██╗ ██║██║   ██║██║     ██║   ██║   ███████║
██║╚██╔╝██║██║   ██║██║╚██╗██║██║   ██║██║     ██║   ██║   ██╔══██║
██║ ╚═╝ ██║╚██████╔╝██║ ╚████║╚██████╔╝███████╗██║   ██║   ██║  ██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝ ╚══════╝╚═╝   ╚═╝   ╚═╝  ╚═╝


  ░▒▓█ _governor_ █▓▒░
  The task keeper - deadlines and schedule in one place.

  ───────────────────────────────────────────────────────────────
  ▓ OVERVIEW
  **governor** is a MONOLITH **node** written in **Go**.
  ▪ Connects to **concentrator** over WebSocket (`ws://` or `wss://` with optional mTLS)
  ▪ Serves a weekly schedule from CSV and JSON-backed events (deadlines, visibility windows)
  ▪ Answers schedule, event, deadline, uptime, and health traffic on the shared wire

  ───────────────────────────────────────────────────────────────
  ▓ ARCHITECTURE
  ▪ **RUNTIME**: Go 1.25+ (see `go.mod`)
  ▪ **TRANSPORT**: WebSocket (`github.com/MrZloHex/monolink`); optional **mTLS** (`wss://`)
  ▪ **NODE ID**: `GOVERNOR`

  ───────────────────────────────────────────────────────────────
  ▓ FEATURES
  ▪ Static weekly schedule from CSV (weekday, start, end, title, location, tags)
  ▪ GET schedule by weekday (colon-safe wire format)
  ▪ Events: add, list, get by id, remove; persisted to JSON across restarts
  ▪ Uptime reporting
  ▪ Ping/pong health check
  ▪ Auto-reconnect on WebSocket disconnect
  ▪ Graceful shutdown on SIGINT/SIGTERM

  ───────────────────────────────────────────────────────────────
  ▓ REQUIREMENTS
  ▪ Go 1.25+ (see `go.mod`)

  ───────────────────────────────────────────────────────────────
  ▓ BUILD & RUN
  **Build**
  ```sh
  go build -o bin/governor ./cmd/governor
  ```

  **Run**
  ```sh
  ./bin/governor
  ```
  Defaults: hub `ws://localhost:8092`, schedule `weekly_schedule.csv`, events `events.json`, log **info** — see **CONFIGURATION**.

  **Example** (explicit flags)
  ```sh
  ./bin/governor -u ws://localhost:8092 -s weekly_schedule.csv -l info
  ```

  ───────────────────────────────────────────────────────────────
  ▓ CONFIGURATION
  On startup, **governor** loads a `.env` file from the current working directory if it exists (`godotenv`). Missing `.env` is fine; other read errors print a warning to stderr and the process continues. Environment variables supply **defaults for flags**; CLI arguments override them.

  **Environment**
  ▪ `GOVERNOR_WS_URL` — WebSocket hub URL (default `ws://localhost:8092`)
  ▪ `GOVERNOR_TLS_CERT` — client certificate (PEM) for mTLS
  ▪ `GOVERNOR_TLS_KEY` — client private key (PEM) for mTLS
  ▪ `GOVERNOR_TLS_CA` — optional PEM bundle to verify the **hub** server certificate

  **Flags**
  ▪ `-u`, `--url` — hub URL (`GOVERNOR_WS_URL`)
  ▪ `-s`, `--schedule` — weekly schedule CSV (default `weekly_schedule.csv`)
  ▪ `-e`, `--events` — events JSON file (default `events.json`)
  ▪ `-l`, `--log` — `debug`, `info`, `warn`, `error` (default `info`)
  ▪ `--tls-cert` — client certificate PEM (`GOVERNOR_TLS_CERT`)
  ▪ `--tls-key` — client private key PEM (`GOVERNOR_TLS_KEY`)
  ▪ `--tls-ca` — optional CA bundle for hub server cert (`GOVERNOR_TLS_CA`)

  ───────────────────────────────────────────────────────────────
  ▓ PROTOCOL
  Packet format: `<TO>:<VERB>:<NOUN>[:<ARGS>...]:<FROM>`

  Responses: `OK:<NOUN>[:ARGS]` or `ERR:<REASON>[:ARGS]`

  ─── PING ───
  `PING:PING` → `PONG:PONG`

  ─── NEW ───
  `NEW:EVENT:<title>:<date>:<time>[:location][:notes][:visible_from]` → `OK:EVENT:<id>`
  Date `YYYY.MM.DD`, time `HH.MM` or `HH.MM.SS` (local).
  `visible_from` (optional) `YYYY.MM.DD` — date from which this event appears in `GET:DEADLINES`; omit = default (event appears 7 days before deadline).

  ─── STOP ───
  `STOP:EVENT:<id>` → `OK:EVENT:<id>` or `ERR:NAC`

  ─── GET ───
  `GET:UPTIME` → `OK:UPTIME:<duration>`
  `GET:AGENDA[:<YYYY.MM.DD>]` → `OK:AGENDA:<date>:<weekday>[:<entry>...]`
  One day in one reply: that weekday's slots in time order, then the
  deadlines visible on that day, soonest first. Entries are
  `CLASS|<start>|<end>|<title>|<location>` and
  `DUE|<title>|<YYYY.MM.DD>|<days remaining>`; days remaining counts whole
  calendar days, so an event later the same day is `0`. Composed here so a
  constrained client (UKAZ on an ESP8266) needs one round trip, not two.
  `GET:SCHEDULE:<weekday>` → `OK:SCHEDULE[:<slot>...]`
  `GET:EVENTS` → `OK:EVENTS[:<event>...]`
  `GET:EVENT:<id>` → `OK:EVENT:<wire>` or `ERR:NAC`
  `GET:DEADLINES[:day|week|month|year]` → `OK:DEADLINES[:<event>...]`
  No arg: events in their visible window (`visibleStart <= now <= deadline`; default `visibleStart` = 7 days before).
  With period: events whose deadline falls in that calendar window and are already visible.

  Weekday: `MON`, `TUE`, `WED`, `THU`, `FRI`, `SAT`, `SUN`

  Slot format (one arg per slot; colons replaced by dots for wire safety):
  `<Weekday>|<Start>|<End>|<Title>|<Location>|<Tags>`
  e.g. `Mon|10.45|12.10|ТФКП|Б.Хим|Lecture;Math`

  Event format (one arg): `<id>|<title>|<at>|<location>|<notes>|<visible_from>`
  `at` = `YYYY.MM.DD.HH.MM` (colon-safe). `visible_from` = `YYYY.MM.DD` or empty (default 7 days before).

  ───────────────────────────────────────────────────────────────
  ▓ FINAL WORDS
  Know your day.
