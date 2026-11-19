package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// trackAlphaAndBeta records 30m of alpha (twice, 10m and 20m) and 1h of beta.
func trackAlphaAndBeta(s *session) {
	s.ok("start", "alpha")
	s.wait(10 * time.Minute)
	s.ok("start", "beta")
	s.wait(time.Hour)
	s.ok("start", "alpha")
	s.wait(20 * time.Minute)
	s.ok("stop")
}

func TestRemoveAsksThenRemovesOnlyThatProject(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", " YES \n"} {
		s := newSession(t)
		trackAlphaAndBeta(s)
		s.in = answer
		out := s.ok("remove", "alpha")
		want := "remove alpha: 2 entries, 30m0s in total? [y/N] removed alpha\n"
		if out != want {
			t.Fatalf("answer %q: remove output = %q, want %q", answer, out, want)
		}
		s.wantReport(`
2026-11-17  1h0m0s
  beta  1h0m0s

All days  1h0m0s
  beta  1h0m0s
`)
	}
}

func TestRemoveWithoutYesChangesNothing(t *testing.T) {
	for _, answer := range []string{"", "\n", "n\n", "no\n", "yep\n"} {
		s := newSession(t)
		trackAlphaAndBeta(s)
		s.in = answer
		out := s.ok("remove", "alpha")
		if !strings.HasSuffix(out, "[y/N] nothing removed\n") {
			t.Fatalf("answer %q: remove output = %q", answer, out)
		}
		s.wantReport(`
2026-11-17  1h30m0s
  alpha  30m0s
  beta   1h0m0s

All days  1h30m0s
  alpha  30m0s
  beta   1h0m0s
`)
	}
}

func TestRemoveUnknownProjectDoesNotAsk(t *testing.T) {
	s := newSession(t)
	trackAlphaAndBeta(s)
	if out := s.ok("remove", "gamma"); out != "no time tracked for gamma\n" {
		t.Fatalf("remove output = %q", out)
	}
}

func TestRemoveRunningProjectFails(t *testing.T) {
	s := newSession(t)
	trackAlphaAndBeta(s)
	s.ok("start", "alpha")
	s.in = "y\n"
	out, err := s.run("remove", "alpha")
	if err == nil || !strings.Contains(err.Error(), "stop alpha before removing it") {
		t.Fatalf("remove error = %v", err)
	}
	if out != "" {
		t.Fatalf("remove asked while running: %q", out)
	}
}

func TestRemoveNeedsAProject(t *testing.T) {
	s := newSession(t)
	if _, err := s.run("remove"); err == nil {
		t.Fatal("remove with no project: want error")
	}
}

// answerAfter runs change, as another terminal would while the question is
// open, then answers yes.
type answerAfter struct {
	change func()
	done   bool
}

func (a *answerAfter) Read(p []byte) (int, error) {
	if !a.done {
		a.done = true
		a.change()
	}
	return copy(p, "y\n"), io.EOF
}

func TestRemoveChangesNothingIfProjectChangesWhileAsking(t *testing.T) {
	for name, change := range map[string]func(s *session){
		"more time tracked": func(s *session) { s.ok("start", "alpha"); s.wait(time.Minute); s.ok("stop") },
		"started":           func(s *session) { s.ok("start", "alpha") },
	} {
		s := newSession(t)
		trackAlphaAndBeta(s)
		var out bytes.Buffer
		err := run([]string{"remove", "alpha"}, s.path, s.clock, &answerAfter{change: func() { change(s) }}, &out, nil)
		if err == nil {
			t.Fatalf("%s: remove succeeded: %q", name, out.String())
		}
		if r := s.ok("report"); !strings.Contains(r, "alpha") {
			t.Fatalf("%s: alpha was removed:\n%s", name, r)
		}
	}
}

func TestRemoveSaysEntryForOne(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	s.ok("stop")
	s.in = "y\n"
	if out := s.ok("remove", "alpha"); out != "remove alpha: 1 entry, 1m0s in total? [y/N] removed alpha\n" {
		t.Fatalf("remove output = %q", out)
	}
}
