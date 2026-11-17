package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseRepo is a clone of a local bare "origin", isolated from the user's git config.
type releaseRepo struct {
	t      *testing.T
	dir    string
	origin string
	script string
}

func newReleaseRepo(t *testing.T) *releaseRepo {
	t.Helper()
	script, err := filepath.Abs("scripts/release.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	r := &releaseRepo{t: t, dir: filepath.Join(root, "work"), origin: filepath.Join(root, "origin.git"), script: script}
	r.git(root, "init", "-q", "--bare", "-b", "main", r.origin)
	r.git(root, "clone", "-q", r.origin, r.dir)
	r.git(r.dir, "checkout", "-q", "-b", "main")
	r.commitAndPush("initial")
	return r
}

func (r *releaseRepo) env() []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
}

func (r *releaseRepo) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, r.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *releaseRepo) commit(msg string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, "file.txt"), []byte(msg), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git(r.dir, "add", "file.txt")
	r.git(r.dir, "commit", "-q", "-m", msg)
}

func (r *releaseRepo) commitAndPush(msg string) {
	r.t.Helper()
	r.commit(msg)
	r.git(r.dir, "push", "-q", "origin", "main")
}

// release runs the release script the way `mise run release [level]` does.
func (r *releaseRepo) release(args ...string) (string, error) {
	cmd := exec.Command("bash", append([]string{r.script}, args...)...)
	cmd.Dir, cmd.Env = r.dir, r.env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *releaseRepo) mustRelease(args ...string) string {
	r.t.Helper()
	out, err := r.release(args...)
	if err != nil {
		r.t.Fatalf("release %v: %v\n%s", args, err, out)
	}
	return out
}

func (r *releaseRepo) mustFail(wantMsg string, args ...string) {
	r.t.Helper()
	out, err := r.release(args...)
	if err == nil || !strings.Contains(out, wantMsg) {
		r.t.Fatalf("release %v: want failure mentioning %q, got err=%v\n%s", args, wantMsg, err, out)
	}
}

// originTags lists the release tags the remote has.
func (r *releaseRepo) originTags() string {
	return strings.Join(strings.Fields(r.git(r.dir, "ls-remote", "--tags", "--refs", "origin")), " ")
}

func (r *releaseRepo) wantOriginTags(tags ...string) {
	r.t.Helper()
	got := r.originTags()
	for _, tag := range tags {
		if !strings.Contains(got, "refs/tags/"+tag) {
			r.t.Fatalf("origin missing %s; has: %s", tag, got)
		}
	}
	if n := strings.Count(got, "refs/tags/"); n != len(tags) {
		r.t.Fatalf("origin has %d tags, want %d: %s", n, len(tags), got)
	}
}

func TestReleaseBumpsPatchMinorAndMajor(t *testing.T) {
	r := newReleaseRepo(t)
	if out := r.mustRelease(); !strings.Contains(out, "released v0.0.1") {
		t.Fatalf("first release: %s", out)
	}
	steps := []struct{ level, want string }{
		{"minor", "v0.1.0"},
		{"patch", "v0.1.1"},
		{"major", "v1.0.0"},
		{"minor", "v1.1.0"},
		{"", "v1.1.1"}, // default is patch
	}
	for _, s := range steps {
		r.commitAndPush("change for " + s.want)
		var args []string
		if s.level != "" {
			args = []string{s.level}
		}
		if out := r.mustRelease(args...); !strings.Contains(out, "released "+s.want) {
			t.Fatalf("release %q: want %s, got %s", s.level, s.want, out)
		}
	}
	r.wantOriginTags("v0.0.1", "v0.1.0", "v0.1.1", "v1.0.0", "v1.1.0", "v1.1.1")
	if kind := r.git(r.dir, "cat-file", "-t", "v1.1.1"); kind != "tag" {
		t.Fatalf("v1.1.1 is a %s, want an annotated tag", kind)
	}
}

func TestReleaseUsesHighestVersionNotNewestTag(t *testing.T) {
	r := newReleaseRepo(t)
	r.git(r.dir, "tag", "v10.0.0")
	r.commit("old line fix")
	r.git(r.dir, "tag", "v9.9.9") // newer tag, lower version, sorts higher as text
	r.git(r.dir, "tag", "v11.0.0-rc1")
	r.commitAndPush("next")
	r.git(r.dir, "tag", "nightly") // not a release, even on HEAD
	if out := r.mustRelease("minor"); !strings.Contains(out, "released v10.1.0") {
		t.Fatalf("want v10.1.0 after v10.0.0, ignoring v9.9.9, v11.0.0-rc1 and nightly: %s", out)
	}
}

func TestReleaseTwiceDoesNotTagAgain(t *testing.T) {
	r := newReleaseRepo(t)
	r.mustRelease()
	if out := r.mustRelease("major"); !strings.Contains(out, "already released as v0.0.1") {
		t.Fatalf("second release: %s", out)
	}
	r.wantOriginTags("v0.0.1")
}

func TestReleaseFinishesAnUnpushedTag(t *testing.T) {
	r := newReleaseRepo(t)
	r.git(r.dir, "tag", "-a", "v0.4.0", "-m", "Release v0.4.0") // tagged, push never happened
	if out := r.mustRelease(); !strings.Contains(out, "already released as v0.4.0") {
		t.Fatalf("rerun: %s", out)
	}
	r.wantOriginTags("v0.4.0")
}

func TestReleaseRefusesUnsafeStates(t *testing.T) {
	t.Run("unknown level", func(t *testing.T) {
		newReleaseRepo(t).mustFail("usage: mise run release [patch|minor|major]", "huge")
	})
	t.Run("uncommitted changes", func(t *testing.T) {
		r := newReleaseRepo(t)
		if err := os.WriteFile(filepath.Join(r.dir, "file.txt"), []byte("dirty"), 0o644); err != nil {
			t.Fatal(err)
		}
		r.mustFail("uncommitted changes")
		r.wantOriginTags()
	})
	t.Run("not on main", func(t *testing.T) {
		r := newReleaseRepo(t)
		r.git(r.dir, "checkout", "-q", "-b", "feature")
		r.mustFail("releases are made from main")
		r.wantOriginTags()
	})
	t.Run("no origin remote", func(t *testing.T) {
		r := newReleaseRepo(t)
		r.git(r.dir, "remote", "remove", "origin")
		r.mustFail("no origin remote")
	})
	t.Run("commit not pushed", func(t *testing.T) {
		r := newReleaseRepo(t)
		r.commit("local only")
		r.mustFail("push main first")
		r.wantOriginTags()
	})
}
