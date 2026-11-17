package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// watcher runs the full-screen view against a session, fed by fake keys and ticks.
type watcher struct {
	keys  chan byte
	ticks chan time.Time
	quit  chan os.Signal
	out   bytes.Buffer
	done  chan watchResult
}

type watchResult struct {
	stopped *Entry
	err     error
}

func (s *session) active() Active {
	s.t.Helper()
	st, err := load(s.path)
	if err != nil || st.Active == nil {
		s.t.Fatalf("no active session: %v", err)
	}
	return *st.Active
}

func (s *session) watch() *watcher { return s.watchAs(os.Getpid()) }

// watchAs runs the view as if it were process pid.
func (s *session) watchAs(pid int) *watcher {
	w := &watcher{keys: make(chan byte), ticks: make(chan time.Time), quit: make(chan os.Signal), done: make(chan watchResult)}
	v := timerView{
		path: s.path, session: s.active(), pid: pid,
		keys: w.keys, ticks: w.ticks, quit: w.quit,
		now: s.clock, screen: screen{Writer: &w.out},
	}
	go func() {
		e, err := watch(v)
		w.done <- watchResult{e, err}
	}()
	return w
}

// tick delivers one timer tick and waits until the view has handled it.
func (w *watcher) tick(now time.Time) {
	w.ticks <- now
	w.keys <- 'x' // ignored; received only once the tick is fully handled
}

// deadPID returns the ID of a process that has already exited.
func deadPID(t *testing.T) int {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func (w *watcher) result(t *testing.T) watchResult {
	t.Helper()
	select {
	case r := <-w.done:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not return")
		return watchResult{}
	}
}

func TestWatchShowsProjectAndTimerAndStopsOnQ(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(90 * time.Second)
	w := s.watch()
	w.keys <- 'q'
	r := w.result(t)
	if r.err != nil || r.stopped == nil || r.stopped.End.Sub(r.stopped.Start) != 90*time.Second {
		t.Fatalf("watch result = %+v", r)
	}
	if out := w.out.String(); !strings.Contains(out, "alpha") || !strings.Contains(out, "00:01:30") {
		t.Fatalf("screen does not show project and timer:\n%q", out)
	}
	if out := s.ok("stop"); out != "nothing to stop\n" {
		t.Fatalf("session still running after q: %q", out)
	}
}

func TestWatchUpdatesTimerAndIgnoresOtherKeys(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watch()
	w.keys <- 'x'
	s.wait(time.Hour + 2*time.Minute + 3*time.Second)
	w.ticks <- s.clock()
	w.keys <- 3 // Ctrl+C
	if r := w.result(t); r.stopped == nil {
		t.Fatalf("Ctrl+C did not stop: %+v", r)
	}
	if out := w.out.String(); !strings.Contains(out, "01:02:03") {
		t.Fatalf("timer not updated:\n%q", out)
	}
}

func TestWatchExitsWhenStoppedElsewhere(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watch()
	s.wait(time.Minute)
	s.ok("stop")
	w.ticks <- s.clock()
	if r := w.result(t); r.err != nil || r.stopped != nil {
		t.Fatalf("watch result = %+v", r)
	}
}

func TestWatchDoesNotStopAProjectStartedElsewhere(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watch()
	s.wait(time.Minute)
	s.ok("start", "beta")
	s.wait(time.Minute)
	w.keys <- 'q'
	if r := w.result(t); r.err != nil || r.stopped != nil {
		t.Fatalf("watch result = %+v", r)
	}
	s.wantReport(`
2026-11-17  2m0s
  alpha  1m0s
  beta   1m0s (running)

All days  2m0s
  alpha  1m0s
  beta   1m0s (running)
`)
}

func TestWatchStopsWhenInputCloses(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(time.Minute)
	w := s.watch()
	close(w.keys) // the terminal went away
	if r := w.result(t); r.err != nil || r.stopped == nil {
		t.Fatalf("watch result = %+v", r)
	}
	if out := s.ok("stop"); out != "nothing to stop\n" {
		t.Fatalf("session still running: %q", out)
	}
}

func TestWatchStopsAndSavesOnSignal(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT} {
		s := newSession(t)
		s.ok("start", "alpha")
		s.wait(time.Minute)
		w := s.watch()
		w.quit <- sig
		if r := w.result(t); r.err != nil || r.stopped == nil {
			t.Fatalf("%v: watch result = %+v", sig, r)
		}
		s.wantReport(`
2026-11-17  1m0s
  alpha  1m0s

All days  1m0s
  alpha  1m0s
`)
	}
}

func TestKilledViewIsStoppedAtItsLastHeartbeat(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watchAs(deadPID(t)) // the view will "die" without running any cleanup
	s.wait(2 * time.Minute)
	w.tick(s.clock())
	s.wait(10 * time.Minute) // no heartbeats: the view was killed
	s.wantReport(`
2026-11-17  2m0s
  alpha  2m0s

All days  2m0s
  alpha  2m0s
`)
	if out := s.ok("stop"); out != "nothing to stop\n" {
		t.Fatalf("stop after recovery = %q", out)
	}
	if files, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), "*")); len(files) != 1 {
		t.Fatalf("leftover files after recovery: %v", files)
	}
	w.quit <- syscall.SIGTERM // let the abandoned goroutine finish; nothing to stop
	if r := w.result(t); r.stopped != nil {
		t.Fatalf("abandoned view stopped something: %+v", r)
	}
}

func TestLiveViewKeepsSessionRunning(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watch()
	s.wait(2 * time.Minute)
	w.tick(s.clock())
	s.wait(10 * time.Minute)
	s.wantReport(`
2026-11-17  12m0s
  alpha  12m0s (running)

All days  12m0s
  alpha  12m0s (running)
`)
	w.keys <- 'q'
	if r := w.result(t); r.stopped == nil {
		t.Fatalf("watch result = %+v", r)
	}
	if files, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), "*")); len(files) != 1 {
		t.Fatalf("view left files behind: %v", files)
	}
}

func TestStartFollowsTheActiveSession(t *testing.T) {
	s := newSession(t)
	var followed []Active
	s.follow = func(a Active) error { followed = append(followed, a); return nil }
	s.ok("start", "alpha")
	s.wait(time.Minute)
	s.ok("start", "alpha") // already tracking: reattach to the same session
	want := Active{Project: "alpha", Start: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)}
	if len(followed) != 2 || !followed[0].Start.Equal(want.Start) || followed[1] != followed[0] || followed[0].Project != want.Project {
		t.Fatalf("followed = %+v", followed)
	}
}

func TestWatchCentersTextOnTheScreen(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	a := s.active()
	var out bytes.Buffer
	keys := make(chan byte, 1)
	keys <- 'q'
	scr := screen{Writer: &out, Size: func() (int, int) { return 40, 9 }}
	v := timerView{path: s.path, session: a, pid: os.Getpid(), keys: keys, now: s.clock, screen: scr}
	if _, err := watch(v); err != nil {
		t.Fatal(err)
	}
	// 3 lines on 9 rows start at row 4; "alpha" is 5 wide on 40 columns, "00:00:00" is 8.
	for _, want := range []string{"\x1b[4;18Halpha", "\x1b[5;17H00:00:00", "\x1b[6;13Hpress q to stop"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("screen missing %q:\n%q", want, out.String())
		}
	}
}

func TestWatchTimerNeverShowsNegativeTime(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	s.wait(-time.Minute) // system clock corrected backward
	w := s.watch()
	w.keys <- 'q'
	w.result(t)
	if out := w.out.String(); !strings.Contains(out, "00:00:00") {
		t.Fatalf("want 00:00:00:\n%q", out)
	}
}

func TestWatchExitsWhenSameProjectIsRestartedElsewhere(t *testing.T) {
	s := newSession(t)
	s.ok("start", "alpha")
	w := s.watch()
	s.wait(time.Minute)
	s.ok("stop")
	s.ok("start", "alpha") // a new session with the same project
	w.ticks <- s.clock()
	if r := w.result(t); r.err != nil || r.stopped != nil {
		t.Fatalf("watch result = %+v", r)
	}
	s.active() // the new session is still running
}
