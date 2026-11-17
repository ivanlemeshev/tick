// Command tick is a minimal time tracker.
//
//	tick start <project>   start tracking a project (switches if another is running)
//	tick stop              stop the running project
//	tick report            show total time per project
//
// Data lives in $TICK_FILE, or ~/.tick.json by default.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const usage = "usage: tick start <project> | tick stop | tick report"

var errUsage = errors.New(usage)

func main() {
	path, err := dataPath()
	if err == nil {
		err = run(os.Args[1:], path, time.Now, os.Stdout, terminalFollower(path, os.Stdout))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tick:", err)
		os.Exit(1)
	}
}

func dataPath() (string, error) {
	if p := os.Getenv("TICK_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tick.json"), nil
}

// run executes one command. After start, follow (if not nil) is shown the active session.
func run(args []string, path string, now func() time.Time, out io.Writer, follow func(Active) error) error {
	if len(args) == 0 {
		return errUsage
	}
	cmd, rest := args[0], args[1:]
	switch {
	case cmd == "start":
		p, err := ParseProject(rest)
		if err != nil {
			return fmt.Errorf("%w\n%s", err, usage)
		}
		var msg string
		next, err := update(path, func(s State) (n State) { n, msg = start(s, p, now()); return n })
		if err != nil {
			return err
		}
		if err := write(out, msg); err != nil || follow == nil {
			return err
		}
		return follow(*next.Active)
	case cmd == "stop" && len(rest) == 0:
		var msg string
		if _, err := update(path, func(s State) (n State) { n, msg = stop(s, now()); return n }); err != nil {
			return err
		}
		return write(out, msg)
	case cmd == "report" && len(rest) == 0:
		s, err := load(path)
		if err != nil {
			return err
		}
		return write(out, report(s, now()))
	}
	return errUsage
}

// write is the one place command output reaches the user.
func write(out io.Writer, msg string) error {
	_, err := io.WriteString(out, msg)
	return err
}

func start(s State, p Project, now time.Time) (State, string) {
	next, stopped, started := Start(s, p, now)
	if started {
		return next, stoppedMsg(stopped) + fmt.Sprintf("started %s\n", p)
	}
	return next, fmt.Sprintf("already tracking %s\n", p)
}

func stop(s State, now time.Time) (State, string) {
	next, stopped := Stop(s, now)
	if stopped == nil {
		return next, "nothing to stop\n"
	}
	return next, stoppedMsg(stopped)
}

func stoppedMsg(e *Entry) string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("stopped %s after %s\n", e.Project, human(e.End.Sub(e.Start)))
}

func report(s State, now time.Time) string {
	r := BuildReport(s, now)
	if len(r.Days) == 0 {
		return "no time tracked\n"
	}
	var b strings.Builder
	for _, d := range r.Days {
		writeSummary(&b, d.Date.Format(time.DateOnly), d.Summary)
		b.WriteString("\n")
	}
	writeSummary(&b, "All days", r.All)
	return b.String()
}

// writeSummary writes a title with the total, then one aligned line per project.
func writeSummary(b *strings.Builder, title string, s Summary) {
	fmt.Fprintf(b, "%s  %s\n", title, human(s.Total))
	width := 0
	for _, l := range s.Lines {
		width = max(width, utf8.RuneCountInString(string(l.Project)))
	}
	for _, l := range s.Lines {
		suffix := ""
		if l.Running {
			suffix = " (running)"
		}
		fmt.Fprintf(b, "  %-*s  %s%s\n", width, l.Project, human(l.Total), suffix)
	}
}

// human formats a duration for display, to the second.
func human(d time.Duration) time.Duration { return d.Round(time.Second) }
