package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// load reads state from path, closing a session whose timer view was killed.
func load(path string) (State, error) {
	s, _, err := loadWithHeartbeats(path)
	return s, err
}

// loadWithHeartbeats reads state and every heartbeat file, keyed by file path,
// and applies Recover.
func loadWithHeartbeats(path string) (State, map[string]Heartbeat, error) {
	s, err := readState(path)
	if err != nil {
		return s, nil, err
	}
	hbs, err := readHeartbeats(path)
	if err != nil {
		return s, nil, err
	}
	return Recover(s, values(hbs)), hbs, nil
}

// update loads state, applies f, and saves and returns the result.
// Heartbeats of dead views are removed once the recovered state is saved.
func update(path string, f func(State) State) (State, error) {
	s, hbs, err := loadWithHeartbeats(path)
	if err != nil {
		return s, err
	}
	next := f(s)
	if err := save(path, next); err != nil {
		return next, err
	}
	for file, hb := range hbs {
		if !hb.Alive {
			os.Remove(file)
		}
	}
	return next, nil
}

// readState reads state from path as stored. A missing file is an empty state.
func readState(path string) (State, error) {
	var s State
	return s, readJSON(path, &s)
}

// heartbeatPath is the file only the view in process pid writes.
func heartbeatPath(path string, pid int) string {
	return path + ".view." + strconv.Itoa(pid)
}

func writeHeartbeat(path string, hb Heartbeat) error {
	return save(heartbeatPath(path, hb.PID), hb)
}

func readHeartbeats(path string) (map[string]Heartbeat, error) {
	dir, prefix := filepath.Dir(path), filepath.Base(path)+".view."
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hbs := map[string]Heartbeat{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		file := filepath.Join(dir, e.Name())
		var hb Heartbeat
		if err := readJSON(file, &hb); err != nil {
			return nil, err
		}
		hb.Alive = processAlive(hb.PID)
		hbs[file] = hb
	}
	return hbs, nil
}

// processAlive reports whether a process with this ID exists.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func values(m map[string]Heartbeat) []Heartbeat {
	vs := make([]Heartbeat, 0, len(m))
	for _, v := range m {
		vs = append(vs, v)
	}
	return vs
}

// readJSON decodes the file at path into v. A missing file leaves v unchanged.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

// save writes v as JSON atomically: a crash leaves either the old or the new file.
func save(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tick-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
