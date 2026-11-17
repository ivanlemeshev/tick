package main

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Project is a validated, non-empty project name. Build it with ParseProject.
type Project string

var errEmptyProject = errors.New("project name must not be empty")

// ParseProject joins words into a project name and rejects blank input.
func ParseProject(words []string) (Project, error) {
	name := strings.Join(strings.Fields(strings.Join(words, " ")), " ")
	if name == "" {
		return "", errEmptyProject
	}
	return Project(name), nil
}

// Entry is one finished stretch of tracked time.
type Entry struct {
	Project Project   `json:"project"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

// Active is the session currently being tracked.
type Active struct {
	Project Project   `json:"project"`
	Start   time.Time `json:"start"`
}

// State holds every finished entry and at most one active session.
type State struct {
	Entries []Entry `json:"entries"`
	Active  *Active `json:"active,omitempty"`
}

// Stop ends the active session. It returns the finished entry, if any.
func Stop(s State, now time.Time) (State, *Entry) {
	if s.Active == nil {
		return s, nil
	}
	e := Entry{Project: s.Active.Project, Start: s.Active.Start, End: now}
	return State{Entries: append(slices.Clone(s.Entries), e)}, &e
}

// Is reports whether a and b are the same session.
func (a Active) Is(b Active) bool {
	return a.Project == b.Project && a.Start.Equal(b.Start)
}

// StopSession stops a only if it is still the active session.
func StopSession(s State, a Active, now time.Time) (State, *Entry) {
	if s.Active == nil || !s.Active.Is(a) {
		return s, nil
	}
	return Stop(s, now)
}

// Start begins tracking p, stopping any other active project first.
// Starting the project already being tracked changes nothing.
func Start(s State, p Project, now time.Time) (next State, stopped *Entry, started bool) {
	if s.Active != nil && s.Active.Project == p {
		return s, nil, false
	}
	next, stopped = Stop(s, now)
	next.Active = &Active{Project: p, Start: now}
	return next, stopped, true
}

// Line is one project's total within a summary.
type Line struct {
	Project Project
	Total   time.Duration
	Running bool
}

// Summary totals time per project, sorted by project name.
type Summary struct {
	Lines []Line
	Total time.Duration
}

// Day is the summary of one calendar day.
type Day struct {
	Date time.Time // local midnight
	Summary
}

// Report is the summary of each day, oldest first, and of all days together.
type Report struct {
	Days []Day
	All  Summary
}

// piece is the part of an entry that falls within one calendar day.
type piece struct {
	project Project
	day     time.Time
	length  time.Duration
	running bool
}

// BuildReport summarizes tracked time, counting the active session up to now.
// Days start at midnight in now's time zone.
func BuildReport(s State, now time.Time) Report {
	finished, _ := Stop(s, now)
	var pieces []piece
	for i, e := range finished.Entries {
		ps := split(e, now.Location())
		if s.Active != nil && i == len(finished.Entries)-1 {
			ps[len(ps)-1].running = true
		}
		pieces = append(pieces, ps...)
	}
	slices.SortStableFunc(pieces, func(a, b piece) int { return a.day.Compare(b.day) })

	var r Report
	for rest := pieces; len(rest) > 0; {
		n := 1
		for n < len(rest) && rest[n].day.Equal(rest[0].day) {
			n++
		}
		r.Days = append(r.Days, Day{Date: rest[0].day, Summary: summarize(rest[:n])})
		rest = rest[n:]
	}
	r.All = summarize(pieces)
	return r
}

// split cuts an entry at each local midnight it crosses.
// It always returns at least one piece, so a zero-length entry still shows.
func split(e Entry, loc *time.Location) []piece {
	var ps []piece
	from, end := e.Start.In(loc), e.End.In(loc)
	for {
		y, m, d := from.Date()
		day := time.Date(y, m, d, 0, 0, 0, 0, loc)
		to := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		if !to.Before(end) {
			return append(ps, piece{project: e.Project, day: day, length: end.Sub(from)})
		}
		ps = append(ps, piece{project: e.Project, day: day, length: to.Sub(from)})
		from = to
	}
}

func summarize(pieces []piece) Summary {
	var sum Summary
	index := map[Project]int{}
	for _, p := range pieces {
		i, ok := index[p.project]
		if !ok {
			i = len(sum.Lines)
			index[p.project] = i
			sum.Lines = append(sum.Lines, Line{Project: p.project})
		}
		sum.Lines[i].Total += p.length
		sum.Lines[i].Running = sum.Lines[i].Running || p.running
		sum.Total += p.length
	}
	slices.SortFunc(sum.Lines, func(a, b Line) int { return strings.Compare(string(a.Project), string(b.Project)) })
	return sum
}
