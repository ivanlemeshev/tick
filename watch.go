package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const ctrlC = 3

// screen is where the full-screen view draws. Size may be nil for a fixed 80x24.
type screen struct {
	io.Writer
	Size func() (width, height int)
}

// watch shows session a full screen until the user quits or a stops elsewhere.
// q or Ctrl+C stops a, if it is still active, and returns the finished entry.
// Each tick redraws and rereads the data file. Closed keys leave a running.
func watch(path string, a Active, keys <-chan byte, ticks <-chan time.Time, now func() time.Time, scr screen) (*Entry, error) {
	for {
		w, h := 80, 24
		if scr.Size != nil {
			w, h = scr.Size()
		}
		fmt.Fprint(scr, frame(a.Project, now().Sub(a.Start), w, h))
		select {
		case k, ok := <-keys:
			if !ok {
				return nil, nil
			}
			if k == 'q' || k == ctrlC {
				return stopSession(path, a, now())
			}
		case <-ticks:
			s, err := load(path)
			if err != nil || s.Active == nil || !s.Active.Is(a) {
				return nil, err
			}
		}
	}
}

func stopSession(path string, a Active, now time.Time) (*Entry, error) {
	var stopped *Entry
	_, err := update(path, func(s State) State {
		next, e := StopSession(s, a, now)
		stopped = e
		return next
	})
	return stopped, err
}

// frame draws the project, a timer and a hint, centered on a cleared screen.
func frame(p Project, elapsed time.Duration, width, height int) string {
	lines := []string{string(p), clock(elapsed), "press q to stop"}
	var b strings.Builder
	b.WriteString("\x1b[2J")
	top := max((height-len(lines))/2, 0)
	for i, l := range lines {
		left := max((width-utf8.RuneCountInString(l))/2, 0)
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", top+i+1, left+1, l)
	}
	return b.String()
}

// clock formats an elapsed duration as HH:MM:SS.
func clock(d time.Duration) string {
	s := int(max(d, 0) / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s/60%60, s%60)
}
