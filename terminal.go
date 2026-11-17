package main

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

// terminalFollower returns a follow hook that opens the full-screen view,
// or nil when stdin or stdout is not a terminal.
func terminalFollower(path string, out io.Writer) func(Active) error {
	in, stdout := int(os.Stdin.Fd()), int(os.Stdout.Fd())
	if !term.IsTerminal(in) || !term.IsTerminal(stdout) {
		return nil
	}
	return func(a Active) error {
		e, err := fullScreen(path, a, in, stdout)
		return errors.Join(err, write(out, stoppedMsg(e)))
	}
}

// fullScreen runs watch in the alternate screen with raw input, restoring both on return.
func fullScreen(path string, a Active, in, stdout int) (stopped *Entry, err error) {
	old, err := term.MakeRaw(in)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, term.Restore(in, old)) }()
	if err := write(os.Stdout, "\x1b[?1049h\x1b[?25l"); err != nil { // alternate screen, hide cursor
		return nil, err
	}
	defer func() { err = errors.Join(err, write(os.Stdout, "\x1b[?25h\x1b[?1049l")) }()

	keys := make(chan byte)
	go readKeys(os.Stdin, keys)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(quit)
	size := func() (int, int) {
		w, h, err := term.GetSize(stdout)
		if err != nil {
			return 80, 24
		}
		return w, h
	}
	return watch(timerView{
		path: path, session: a, pid: os.Getpid(),
		keys: keys, ticks: ticker.C, quit: quit, now: time.Now, screen: screen{os.Stdout, size},
	})
}

func readKeys(r io.Reader, keys chan<- byte) {
	defer close(keys)
	buf := make([]byte, 1)
	for {
		if _, err := r.Read(buf); err != nil {
			return
		}
		keys <- buf[0]
	}
}
