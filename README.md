# tick

A minimal command-line time tracker.

```
go install github.com/ivanlemeshev/tick@latest

tick start <project>   # start tracking; switches if another project is running
tick stop              # stop the running project
tick report            # total time per project, including the running one
```

Data is stored as JSON in `$TICK_FILE`, or `~/.tick.json` by default. Writes are
atomic, so a crash leaves either the old file or the new one.

## Glossary

- **Project**: a non-empty name, trimmed, inner whitespace collapsed. `tick start my project` tracks `my project`.
- **Active session**: the one project being tracked right now. There is at most one.
- **Entry**: a finished stretch of time for a project, with a start and an end.
- **Switch**: `tick start` on a different project while one is active stops the active one first.
- **Report**: the sum of entries per project, plus the active session up to now, sorted by name and rounded to the second.

Running `start` on the project already active, or `stop` when nothing is active, changes nothing.
