package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Commands from separate terminals write the same data file at once.
// Every switch must be kept: n starts leave n-1 entries and one active session.
func TestConcurrentStartsKeepEverySession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tick.json")
	const n = 40
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			if err := run([]string{"start", fmt.Sprint("p", i)}, path, time.Now, io.Discard, nil); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	s, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != n-1 || s.Active == nil {
		t.Fatalf("lost sessions: %d entries, active %v; want %d entries and one active", len(s.Entries), s.Active, n-1)
	}
}
