package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// session drives the CLI the way a user does: same data file, advancing clock.
type session struct {
	t    *testing.T
	path string
	now  time.Time
}

func newSession(t *testing.T) *session {
	return &session{
		t:    t,
		path: filepath.Join(t.TempDir(), "tick.json"),
		now:  time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC),
	}
}

func (s *session) run(args ...string) (string, error) {
	var out bytes.Buffer
	err := run(args, s.path, func() time.Time { return s.now }, &out)
	return out.String(), err
}

func (s *session) ok(args ...string) string {
	s.t.Helper()
	out, err := s.run(args...)
	if err != nil {
		s.t.Fatalf("tick %v: unexpected error: %v", args, err)
	}
	return out
}

func (s *session) wait(d time.Duration) { s.now = s.now.Add(d) }

func TestStartStopReport(t *testing.T) {
	s := newSession(t)
	if out := s.ok("start", "alpha"); out != "started alpha\n" {
		t.Fatalf("start output = %q", out)
	}
	s.wait(90 * time.Minute)
	if out := s.ok("stop"); out != "stopped alpha after 1h30m0s\n" {
		t.Fatalf("stop output = %q", out)
	}
	s.wait(time.Hour)
	if out := s.ok("report"); out != "alpha  1h30m0s\n" {
		t.Fatalf("report output = %q", out)
	}
}

func TestReportSumsPerProjectSortedAndIncludesRunning(t *testing.T) {
	s := newSession(t)
	s.ok("start", "beta")
	s.wait(10 * time.Minute)
	s.ok("stop")
	s.ok("start", "alpha")
	s.wait(20 * time.Minute)
	s.ok("stop")
	s.ok("start", "beta")
	s.wait(5 * time.Minute)

	want := "alpha  20m0s\nbeta   15m0s (running)\n"
	if out := s.ok("report"); out != want {
		t.Fatalf("report output =\n%q\nwant\n%q", out, want)
	}
}

func TestStartingAnotherProjectSwitches(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	if out := s.ok("start", "beta"); out != "stopped alpha after 1m0s\nstarted beta\n" {
		t.Fatalf("switch output = %q", out)
	}
	s.wait(2 * time.Minute)
	want := "alpha  1m0s\nbeta   2m0s (running)\n"
	if out := s.ok("report"); out != want {
		t.Fatalf("report output = %q, want %q", out, want)
	}
}

func TestStartIsIdempotent(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	if out := s.ok("start", "alpha"); out != "already tracking alpha\n" {
		t.Fatalf("second start output = %q", out)
	}
	s.wait(time.Minute)
	if out := s.ok("report"); out != "alpha  2m0s (running)\n" {
		t.Fatalf("report output = %q", out)
	}
}

func TestStopWhenIdleIsIdempotent(t *testing.T) {
	s := newSession(t)
	for range 2 {
		if out := s.ok("stop"); out != "nothing to stop\n" {
			t.Fatalf("stop output = %q", out)
		}
	}
}

func TestReportWhenEmpty(t *testing.T) {
	s := newSession(t)
	if out := s.ok("report"); out != "no time tracked\n" {
		t.Fatalf("report output = %q", out)
	}
}

func TestProjectNameIsTrimmedAndMayHaveSpaces(t *testing.T) {
	s := newSession(t)
	if out := s.ok("start", "  my", "project "); out != "started my project\n" {
		t.Fatalf("start output = %q", out)
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{},
		{"start"},
		{"start", "   "},
		{"stop", "extra"},
		{"report", "extra"},
		{"bogus"},
	}
	for _, args := range cases {
		s := newSession(t)
		_, err := s.run(args...)
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("tick %q: want usage error, got %v", args, err)
		}
	}
}

func TestFirstStartCreatesDataDirectory(t *testing.T) {
	s := newSession(t)
	s.path = filepath.Join(t.TempDir(), "missing-dir", "tick.json")
	s.ok("start", "alpha")
	s.wait(time.Minute)
	if out := s.ok("report"); out != "alpha  1m0s (running)\n" {
		t.Fatalf("report output = %q", out)
	}
}

func TestUnwritablePathIsReported(t *testing.T) {
	s := newSession(t)
	s.path = "/dev/null/not-a-dir/tick.json"
	if _, err := s.run("start", "x"); err == nil {
		t.Fatal("want error writing to an impossible path")
	}
}

func TestCorruptDataFileIsReportedAndLeftAlone(t *testing.T) {
	s := newSession(t)
	if err := os.WriteFile(s.path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"start", "x"}, {"stop"}, {"report"}} {
		if _, err := s.run(args...); err == nil || !strings.Contains(err.Error(), s.path) {
			t.Errorf("tick %v: want error naming the file, got %v", args, err)
		}
	}
	if data, _ := os.ReadFile(s.path); string(data) != "{not json" {
		t.Fatalf("corrupt file was overwritten: %q", data)
	}
}

func TestDurationsAreShownToTheSecond(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(90*time.Second + 400*time.Millisecond)
	if out := s.ok("report"); out != "alpha  1m30s (running)\n" {
		t.Fatalf("report output = %q", out)
	}
	if out := s.ok("stop"); out != "stopped alpha after 1m30s\n" {
		t.Fatalf("stop output = %q", out)
	}
}
