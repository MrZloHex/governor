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
  **_governor_** is a schedule & task-deadline node written in **Go**.  
  It connects to a _concentrator_ hub via WebSocket,  
  serving a static weekly schedule (e.g. university timetable)  
  and, later, task deadlines. Request a day's schedule or  
  ping it for health. Steady. Structured.  

  ───────────────────────────────────────────────────────────────  
  ▓ ARCHITECTURE  
  ▪ **RUNTIME**: Go 1.25  
  ▪ **TRANSPORT**: WebSocket (gorilla/websocket) via pkg/proto client  
  ▪ **SCHEDULE**: Static weekly CSV, loaded at startup  
  ▪ **NODE ID**: GOVERNOR  

  ───────────────────────────────────────────────────────────────  
  ▓ FEATURES  
  ▪ Static weekly schedule from CSV (weekday, start, end, title, location, tags)  
  ▪ GET schedule by weekday (colon-safe wire format)  
  ▪ Events: add, list, get by id, remove; persisted to JSON file across restarts  
  ▪ Uptime reporting  
  ▪ Ping/pong health check  
  ▪ Auto-reconnect on WebSocket disconnect  
  ▪ Graceful shutdown on SIGINT/SIGTERM  

  ───────────────────────────────────────────────────────────────  
  ▓ BUILD & RUN  
  ```sh  
  go build -o bin/governor ./cmd/governor  
  ./bin/governor -u ws://localhost:8092 -s weekly_schedule.csv -l info  
  ```

  Flags:  
  ▪ `-u`  WebSocket hub URL  (default: ws://localhost:8092)  
  ▪ `-s`  Path to weekly schedule CSV  (default: weekly_schedule.csv)  
  ▪ `-e`  Path to events persistence file (JSON)  (default: events.json)  
  ▪ `-l`  Log level: debug, info, warn, error  (default: info)  

  ───────────────────────────────────────────────────────────────  
  ▓ PROTOCOL  
  Packet format:  <TO>:<VERB>:<NOUN>[:<ARGS>...]:<FROM>  

  Responses:  OK:<NOUN>[:ARGS]  or  ERR:<REASON>[:ARGS]  

  ─── PING ───  
  PING:PING                        -> PONG:PONG  

  ─── NEW ───  
  NEW:EVENT:<title>:<date>:<time>[:location][:notes]  -> OK:EVENT:<id>  
  Date YYYY.MM.DD, time HH.MM or HH.MM.SS (local).  

  ─── STOP ───  
  STOP:EVENT:<id>                  -> OK:EVENT:<id>  or  ERR:NAC  

  ─── GET ───  
  GET:UPTIME                       -> OK:UPTIME:<duration>  
  GET:SCHEDULE:<weekday>           -> OK:SCHEDULE[:<slot>...]  
  GET:EVENTS                       -> OK:EVENTS[:<event>...]  
  GET:EVENT:<id>                   -> OK:EVENT:<wire>  or  ERR:NAC  

  Weekday: Mon, Tue, Wed, Thu, Fri, Sat (case-insensitive).  

  Slot format (one arg per slot; colons replaced by dots for wire safety):  
  <Weekday>|<Start>|<End>|<Title>|<Location>|<Tags>  
  e.g.  Mon|10.45|12.10|ТФКП|Б.Хим|Lecture;Math  

  Event format (one arg):  <id>|<title>|<at>|<location>|<notes>  
  at = YYYY.MM.DD.HH.MM (colon-safe).  

  ───────────────────────────────────────────────────────────────  
  ▓ FINAL WORDS  
  Know your day.  
