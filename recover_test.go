package main

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)

func running(p Project) State { return State{Active: &Active{Project: p, Start: t0}} }

func TestRecoverIgnoresHeartbeatsOfOtherSessions(t *testing.T) {
	// alpha's view was killed right after the user switched to beta elsewhere.
	alpha := Active{Project: "alpha", Start: t0.Add(-time.Hour)}
	s := Recover(running("beta"), []Heartbeat{{Session: alpha, Seen: t0.Add(time.Minute)}})
	if s.Active == nil || s.Active.Project != "beta" || len(s.Entries) != 0 {
		t.Fatalf("beta was not left running: %+v", s)
	}
}

func TestRecoverStopsAtTheLatestHeartbeatOfDeadViews(t *testing.T) {
	a := *running("alpha").Active
	s := Recover(running("alpha"), []Heartbeat{
		{Session: a, Seen: t0.Add(5 * time.Minute)},
		{Session: a, Seen: t0.Add(3 * time.Minute)},
	})
	if s.Active != nil || len(s.Entries) != 1 || !s.Entries[0].End.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("want alpha stopped at 5m: %+v", s)
	}
}

func TestProcessOwnedByAnotherUserIsAlive(t *testing.T) {
	if !processAlive(1) { // launchd or init, owned by root
		t.Fatal("pid 1 reported dead")
	}
}
