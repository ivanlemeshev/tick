# tick

A minimal command-line time tracker.

```
go install github.com/ivanlemeshev/tick@latest

tick start <project>   # start tracking and open the full-screen timer; switches if another project is running
tick stop              # stop the running project
tick report            # time per day and for all days, per project
```

```
2026-11-17  1h30m0s
  alpha  30m0s
  beta   1h0m0s

2026-11-18  10m0s
  beta  10m0s (running)

All days  1h40m0s
  alpha  30m0s
  beta   1h10m0s (running)
```

In a terminal, `tick start` takes over the screen with the project name and a
live timer. Press `q` or Ctrl+C to stop tracking and return to the shell. When
output is piped, `start` just starts tracking and exits. Running `tick start`
on the project already active reopens its timer. Closing the terminal leaves the
session running; `tick stop` ends it later.

Data is stored as JSON in `$TICK_FILE`, or `~/.tick.json` by default. Writes are
atomic, so a crash leaves either the old file or the new one.

## Glossary

- **Project**: a non-empty name, trimmed, inner whitespace collapsed. `tick start my project` tracks `my project`.
- **Active session**: the one project being tracked right now. There is at most one.
- **Entry**: a finished stretch of time for a project, with a start and an end.
- **Timer view**: the full-screen display opened by `tick start` in a terminal. It follows one session: if that session is stopped or replaced from another terminal, the view closes without stopping anything.
- **Switch**: `tick start` on a different project while one is active stops the active one first.
- **Day**: a calendar day in your local time zone. Time that crosses midnight is split between the two days.
- **Summary**: total time per project, sorted by name, plus a total for all projects.
- **Report**: a summary for each day, oldest first, then a summary for all days. It counts the active session up to now, marked `(running)`, and rounds to the second.

Running `start` on the project already active, or `stop` when nothing is active, changes nothing.
