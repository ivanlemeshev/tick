package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// session drives the CLI the way a user does: same data file, advancing clock.
type session struct {
	t      *testing.T
	path   string
	now    time.Time
	follow func(Active) error
	in     string     // what the user types at a prompt
	mu     sync.Mutex // guards now, which a watch goroutine may read
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
	err := run(args, s.path, s.clock, strings.NewReader(s.in), &out, s.follow)
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

func (s *session) clock() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now
}

func (s *session) wait(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = s.now.Add(d)
}

// wantReport checks tick report output; want starts with a newline for readability.
func (s *session) wantReport(want string) {
	s.t.Helper()
	want = strings.TrimPrefix(want, "\n")
	if out := s.ok("report"); out != want {
		s.t.Fatalf("report output:\n%s\nwant:\n%s", out, want)
	}
}

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
	s.wantReport(`
2026-11-17  1h30m0s
  alpha  1h30m0s

All days  1h30m0s
  alpha  1h30m0s
`)
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

	s.wantReport(`
2026-11-17  35m0s
  alpha  20m0s
  beta   15m0s (running)

All days  35m0s
  alpha  20m0s
  beta   15m0s (running)
`)
}

func TestStartingAnotherProjectSwitches(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	if out := s.ok("start", "beta"); out != "stopped alpha after 1m0s\nstarted beta\n" {
		t.Fatalf("switch output = %q", out)
	}
	s.wait(2 * time.Minute)
	s.wantReport(`
2026-11-17  3m0s
  alpha  1m0s
  beta   2m0s (running)

All days  3m0s
  alpha  1m0s
  beta   2m0s (running)
`)
}

func TestStartIsIdempotent(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	if out := s.ok("start", "alpha"); out != "already tracking alpha\n" {
		t.Fatalf("second start output = %q", out)
	}
	s.wait(time.Minute)
	s.wantReport(`
2026-11-17  2m0s
  alpha  2m0s (running)

All days  2m0s
  alpha  2m0s (running)
`)
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
	s.wantReport(`
2026-11-17  1m0s
  alpha  1m0s (running)

All days  1m0s
  alpha  1m0s (running)
`)
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
	s.follow = func(Active) error { t.Fatal("follow called after a failed start"); return nil }
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
	s.wantReport(`
2026-11-17  1m30s
  alpha  1m30s (running)

All days  1m30s
  alpha  1m30s (running)
`)
	if out := s.ok("stop"); out != "stopped alpha after 1m30s\n" {
		t.Fatalf("stop output = %q", out)
	}
}

func TestReportSplitsTimeAtMidnight(t *testing.T) {
	s := newSession(t)
	s.wait(14*time.Hour + 30*time.Minute) // 23:30
	s.ok("start", "alpha")
	s.wait(75 * time.Minute) // 00:45 next day
	s.ok("stop")
	s.wantReport(`
2026-11-17  30m0s
  alpha  30m0s

2026-11-18  45m0s
  alpha  45m0s

All days  1h15m0s
  alpha  1h15m0s
`)
}

func TestReportShowsEachDayOldestFirstAndRunningOnlyToday(t *testing.T) {
	s := newSession(t)
	s.ok("start", "beta")
	s.wait(time.Hour)
	s.ok("start", "alpha")
	s.wait(30 * time.Minute)
	s.ok("stop")
	s.wait(24 * time.Hour)
	s.ok("start", "beta")
	s.wait(10 * time.Minute)
	s.wantReport(`
2026-11-17  1h30m0s
  alpha  30m0s
  beta   1h0m0s

2026-11-18  10m0s
  beta  10m0s (running)

All days  1h40m0s
  alpha  30m0s
  beta   1h10m0s (running)
`)
}

func TestReportUsesTheClockTimeZoneForDays(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha") // 09:00 UTC is 22:00 in UTC-11
	s.wait(3 * time.Hour)
	s.ok("stop")
	s.now = s.now.In(time.FixedZone("UTC-11", -11*60*60))
	s.wantReport(`
2026-11-16  2h0m0s
  alpha  2h0m0s

2026-11-17  1h0m0s
  alpha  1h0m0s

All days  3h0m0s
  alpha  3h0m0s
`)
}

func TestReportRightAfterStartShowsRunningProject(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wantReport(`
2026-11-17  0s
  alpha  0s (running)

All days  0s
  alpha  0s (running)
`)
}

func TestReportOrdersDaysEvenAfterClockMovesBack(t *testing.T) {
	s := newSession(t)
	s.wait(24 * time.Hour)
	s.ok("start", "alpha")
	s.wait(time.Hour)
	s.ok("stop")
	s.wait(-48 * time.Hour) // system clock corrected backward
	s.ok("start", "beta")
	s.wait(time.Hour)
	s.ok("stop")
	s.wantReport(`
2026-11-16  1h0m0s
  beta  1h0m0s

2026-11-18  1h0m0s
  alpha  1h0m0s

All days  2h0m0s
  alpha  1h0m0s
  beta   1h0m0s
`)
}
