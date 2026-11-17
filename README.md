# tick

[![Lint](https://github.com/ivanlemeshev/tick/actions/workflows/lint.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/lint.yml)
[![Test](https://github.com/ivanlemeshev/tick/actions/workflows/test.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/test.yml)
[![Build](https://github.com/ivanlemeshev/tick/actions/workflows/build.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/build.yml)
[![CodeQL](https://github.com/ivanlemeshev/tick/actions/workflows/codeql.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/codeql.yml)

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
on the project already active reopens its timer.

Ending the timer any other way also stops tracking and saves it: closing the
terminal, `kill`, Ctrl+C or a hangup. Even `kill -9` or a crash, which no
program can intercept, loses at most about a second: the timer leaves a
heartbeat each second, and the next `tick` command closes the session at the
last one. Saves are atomic, so a kill during a write never corrupts history.

Data is stored as JSON in `$TICK_FILE`, or `~/.tick.json` by default. Writes are
atomic, so a crash leaves either the old file or the new one. While a timer
is open it also keeps `<data file>.view.<pid>`, removed when it exits.
`<data file>.lock` lets only one command write at a time, so commands run from
several terminals at once never lose each other's changes.

## Glossary

- **Project**: a non-empty name, trimmed, inner whitespace collapsed. `tick start my project` tracks `my project`.
- **Active session**: the one project being tracked right now. There is at most one.
- **Entry**: a finished stretch of time for a project, with a start and an end.
- **Timer view**: the full-screen display opened by `tick start` in a terminal. It follows one session: if that session is stopped or replaced from another terminal, the view closes without stopping anything.
- **Heartbeat**: a timer view's last sign of life, its process ID and time, written each second to its own file.
- **Recovery**: when every view following the active session has died without stopping it, the session ends at the latest heartbeat. A session started without a view has no heartbeat and keeps running until `tick stop`.
- **Switch**: `tick start` on a different project while one is active stops and records the active one, then starts the new one, in a single save. Works from any terminal: an open timer for the old project closes itself within a second.
- **Day**: a calendar day in your local time zone. Time that crosses midnight is split between the two days.
- **Summary**: total time per project, sorted by name, plus a total for all projects.
- **Report**: a summary for each day, oldest first, then a summary for all days. It counts the active session up to now, marked `(running)`, and rounds to the second.

Running `start` on the project already active, or `stop` when nothing is active, changes nothing.

## Development

[mise](https://mise.jdx.dev) pins Go and golangci-lint and runs the tasks:

```
mise install    # install the pinned Go version
mise run test   # all tests with the race detector, printing total coverage
mise run lint   # golangci-lint, configured in .golangci.yml
mise run build  # check that it builds, keeping no binary
```

GitHub Actions runs each of these tasks as its own workflow on every push to
`main` and every pull request, with tool versions taken from `mise.toml`. CodeQL
also scans the code for security issues on each push, pull request and weekly.

## Releasing

Versions are git tags of the form `vMAJOR.MINOR.PATCH`; there is no version file.

```
mise run release          # v1.2.3 -> v1.2.4 (default)
mise run release minor    # v1.2.3 -> v1.3.0
mise run release major    # v1.2.3 -> v2.0.0
```

It bumps the highest existing `vX.Y.Z` tag (starting from `v0.0.0`), creates an
annotated tag and pushes it to `origin`. It only releases from `main` with a
clean working tree, when `HEAD` matches `origin/main`. Running it again on an
already released commit just pushes that tag, so a release whose push failed
can be finished by rerunning it.

## License

MIT. See [LICENSE](LICENSE).
