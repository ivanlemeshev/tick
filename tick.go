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

// Line is one project's total in a report.
type Line struct {
	Project Project
	Total   time.Duration
	Running bool
}

// Report totals time per project, counting the active session up to now.
func Report(s State, now time.Time) []Line {
	finished, _ := Stop(s, now)
	totals := map[Project]time.Duration{}
	for _, e := range finished.Entries {
		totals[e.Project] += e.End.Sub(e.Start)
	}
	lines := make([]Line, 0, len(totals))
	for p, d := range totals {
		running := s.Active != nil && s.Active.Project == p
		lines = append(lines, Line{Project: p, Total: d, Running: running})
	}
	slices.SortFunc(lines, func(a, b Line) int { return strings.Compare(string(a.Project), string(b.Project)) })
	return lines
}
