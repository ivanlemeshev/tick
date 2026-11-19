# tick

[![Format](https://github.com/ivanlemeshev/tick/actions/workflows/format.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/format.yml)
[![Lint](https://github.com/ivanlemeshev/tick/actions/workflows/lint.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/lint.yml)
[![Test](https://github.com/ivanlemeshev/tick/actions/workflows/test.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/test.yml)
[![Build](https://github.com/ivanlemeshev/tick/actions/workflows/build.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/build.yml)
[![CodeQL](https://github.com/ivanlemeshev/tick/actions/workflows/codeql.yml/badge.svg)](https://github.com/ivanlemeshev/tick/actions/workflows/codeql.yml)

A minimal command-line time tracker.

```sh
go install github.com/ivanlemeshev/tick@latest

tick start <project>   # start tracking and open the full-screen timer; switch from the running project
tick stop              # stop the running project
tick report            # show time per project for each day and for all days
tick remove <project>  # delete all time of a project after you confirm
```

```text
2026-11-17  1h30m0s
  alpha  30m0s
  beta   1h0m0s

2026-11-18  10m0s
  beta  10m0s (running)

All days  1h40m0s
  alpha  30m0s
  beta   1h10m0s (running)
```

In a terminal, `tick start` shows the project name and a live timer on the full screen. Press `q` or Ctrl+C to stop tracking and go back to the shell. When the output goes to a pipe, `start` starts tracking and exits. Run `tick start` on the active project to open its timer again.

The timer also stops tracking and saves the time when you close the terminal, send `kill` or a hangup, or press Ctrl+C. No program can catch `kill -9` or a crash, but you lose at most about one second. The timer writes a heartbeat each second, and the next `tick` command ends the session at the last heartbeat.

`start` on the active project and `stop` with no active project change nothing.

`tick remove <project>` shows how many entries and how much time it will delete, then asks `[y/N]`. Only `y` or `yes` deletes. Any other answer, or no input, deletes nothing. It does not remove the running project, so stop the project first. If the project changes in another terminal while the question is open, `remove` deletes nothing and fails. A project with no tracked time is not changed.

## Data

tick stores data as JSON in `$TICK_FILE`, or in `~/.tick.json` by default. Each write is atomic: a crash leaves the old file or the new file, never a damaged one. An open timer also writes `<data file>.view.<pid>` and removes it when it exits. `<data file>.lock` lets only one command write at a time, so commands in different terminals never lose their changes.

## Glossary

- **Project**: a non-empty name. tick joins the words, trims the name and collapses inner whitespace, so `tick start my project` tracks `my project`.
- **Active session**: the project that tick tracks now. There is at most one.
- **Entry**: a finished period of time for a project, with a start and an end.
- **Timer view**: the full-screen display that `tick start` opens in a terminal. It follows one session. If another terminal stops or replaces that session, the view closes and stops nothing.
- **Heartbeat**: the last sign of life from a timer view: its process ID and the time. Each view writes its heartbeat to its own file each second.
- **Recovery**: when all views that follow the active session die without stopping it, the session ends at the latest heartbeat. A session started without a view has no heartbeat, so it runs until `tick stop`.
- **Switch**: `tick start` on a different project stops and records the active project, then starts the new one, in one save. It works from any terminal. An open timer for the old project closes in one second or less.
- **Day**: a calendar day in your local time zone. Time that crosses midnight is split between the two days.
- **Summary**: the total time per project, sorted by name, and the total for all projects.
- **Report**: a summary for each day, oldest first, then a summary for all days. It includes the active session up to now, marked `(running)`, and rounds to the second.

## Development

[mise](https://mise.jdx.dev) pins Go, golangci-lint and the format and lint tools. It also runs the tasks:

```sh
mise install              # install the pinned tools
mise run format           # format Go, Markdown, YAML, TOML and Bash files
mise run format:check     # check formatting without changes
mise run lint             # golangci-lint, markdownlint, shellcheck and tombi
mise run test             # run all tests with the race detector and show coverage
mise run build            # check that it builds and keep no binary
mise run check            # run format:check, lint, test and build
```

GitHub Actions runs `format:check`, `lint`, `test` and `build` as separate workflows on each push to `main` and on each pull request. The workflows use the tool versions from `mise.toml`. CodeQL scans the code for security problems on each push, on each pull request and each week.

## Releasing

Versions are git tags of the form `vMAJOR.MINOR.PATCH`. There is no version file.

```sh
mise run release          # v1.2.3 -> v1.2.4 (default)
mise run release minor    # v1.2.3 -> v1.3.0
mise run release major    # v1.2.3 -> v2.0.0
```

The task increments the highest `vX.Y.Z` tag, or `v0.0.0` if there is no tag. It shows the new version and asks for confirmation. Then it creates an annotated tag and pushes it to `origin`. Any answer other than `y` or `yes` cancels the release. No terminal input also cancels it. The task releases only from `main` with a clean working tree, when `HEAD` matches `origin/main`. On a commit that already has a release tag, the task only pushes that tag. If a push failed, run the task again to finish the release.

A pushed tag starts the Release workflow. The workflow runs the tests. Then [GoReleaser](https://goreleaser.com) builds `tick` for macOS and Linux (amd64 and arm64). It publishes a GitHub Release with the archives, the checksums and release notes for the changes since the previous release.

```sh
mise run release-check   # validate .goreleaser.yaml, build all archives and keep none
```

## License

MIT. See [LICENSE](LICENSE).
