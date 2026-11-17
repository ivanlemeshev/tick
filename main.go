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
	"text/tabwriter"
	"time"
)

const usage = "usage: tick start <project> | tick stop | tick report"

var errUsage = errors.New(usage)

func main() {
	path, err := dataPath()
	if err == nil {
		err = run(os.Args[1:], path, time.Now, os.Stdout)
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

func run(args []string, path string, now func() time.Time, out io.Writer) error {
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
		return update(path, func(s State) State { return start(s, p, now(), out) })
	case cmd == "stop" && len(rest) == 0:
		return update(path, func(s State) State { return stop(s, now(), out) })
	case cmd == "report" && len(rest) == 0:
		s, err := load(path)
		if err != nil {
			return err
		}
		return report(s, now(), out)
	}
	return errUsage
}

// update loads state, applies f, and saves the result.
func update(path string, f func(State) State) error {
	s, err := load(path)
	if err != nil {
		return err
	}
	return save(path, f(s))
}

func start(s State, p Project, now time.Time, out io.Writer) State {
	next, stopped, started := Start(s, p, now)
	printStopped(out, stopped)
	if started {
		fmt.Fprintf(out, "started %s\n", p)
	} else {
		fmt.Fprintf(out, "already tracking %s\n", p)
	}
	return next
}

func stop(s State, now time.Time, out io.Writer) State {
	next, stopped := Stop(s, now)
	if stopped == nil {
		fmt.Fprintln(out, "nothing to stop")
	}
	printStopped(out, stopped)
	return next
}

func printStopped(out io.Writer, e *Entry) {
	if e != nil {
		fmt.Fprintf(out, "stopped %s after %s\n", e.Project, human(e.End.Sub(e.Start)))
	}
}

func report(s State, now time.Time, out io.Writer) error {
	r := BuildReport(s, now)
	if len(r.Days) == 0 {
		_, err := fmt.Fprintln(out, "no time tracked")
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, d := range r.Days {
		writeSummary(w, d.Date.Format(time.DateOnly), d.Summary)
		fmt.Fprintln(w)
	}
	writeSummary(w, "All days", r.All)
	return w.Flush()
}

func writeSummary(w io.Writer, title string, s Summary) {
	fmt.Fprintf(w, "%s  %s\n", title, human(s.Total))
	for _, l := range s.Lines {
		suffix := ""
		if l.Running {
			suffix = " (running)"
		}
		fmt.Fprintf(w, "  %s\t%s%s\n", l.Project, human(l.Total), suffix)
	}
}

// human formats a duration for display, to the second.
func human(d time.Duration) time.Duration { return d.Round(time.Second) }
