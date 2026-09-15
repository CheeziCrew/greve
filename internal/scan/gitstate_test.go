package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeGit builds a .git directory from a path->content map.
func writeGit(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(repo, ".git", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestReadGitState_InSyncWithOrigin(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":                     "ref: refs/heads/main\n",
		"refs/heads/main":          "abc123\n",
		"refs/remotes/origin/main": "abc123\n",
		"refs/remotes/origin/HEAD": "ref: refs/remotes/origin/main\n",
	})
	got := readGitState(repo)
	if !got.InSyncWithOrigin() {
		t.Errorf("same sha on the default branch must be in sync: %+v", got)
	}
	if got.Drifted() {
		t.Error("in-sync clone must not report drift")
	}
}

func TestReadGitState_BehindOrigin(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":                     "ref: refs/heads/main\n",
		"refs/heads/main":          "aaa\n",
		"refs/remotes/origin/main": "bbb\n",
		"refs/remotes/origin/HEAD": "ref: refs/remotes/origin/main\n",
	})
	if got := readGitState(repo); got.InSyncWithOrigin() {
		t.Errorf("different sha must not be in sync: %+v", got)
	}
}

func TestReadGitState_FeatureBranchIsDrift(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":                     "ref: refs/heads/drakel\n",
		"refs/heads/drakel":        "aaa\n",
		"refs/remotes/origin/main": "aaa\n",
		"refs/remotes/origin/HEAD": "ref: refs/remotes/origin/main\n",
	})
	got := readGitState(repo)
	if got.Branch != "drakel" {
		t.Errorf("branch = %q, want drakel", got.Branch)
	}
	// Same sha, but a feature branch can still carry different files.
	if got.InSyncWithOrigin() {
		t.Error("a non-default branch must never count as in sync")
	}
}

func TestReadGitState_DetachedHead(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":                     "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432\n",
		"refs/remotes/origin/main": "aaa\n",
	})
	got := readGitState(repo)
	if !got.Detached {
		t.Error("a non-symbolic HEAD is detached")
	}
	if got.InSyncWithOrigin() {
		t.Error("detached HEAD must never count as in sync")
	}
}

func TestReadGitState_PackedRefs(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD": "ref: refs/heads/main\n",
		"packed-refs": "# pack-refs with: peeled fully-peeled sorted\n" +
			"aaa refs/heads/main\n" +
			"aaa refs/remotes/origin/main\n" +
			"^deadbeef\n",
	})
	got := readGitState(repo)
	if got.HeadSHA != "aaa" || got.OriginSHA != "aaa" {
		t.Errorf("packed-refs not resolved: head=%q origin=%q", got.HeadSHA, got.OriginSHA)
	}
	if !got.InSyncWithOrigin() {
		t.Error("packed refs at the same sha are in sync")
	}
}

func TestReadGitState_DefaultsToMainWithoutOriginHead(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":            "ref: refs/heads/main\n",
		"refs/heads/main": "aaa\n",
	})
	if got := readGitState(repo); got.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main when origin/HEAD is missing", got.DefaultBranch)
	}
}

// A clone that has never fetched must not read as fresh — that is exactly the
// case where "0 commits behind" is a lie.
func TestReadGitState_NeverFetchedIsNotFresh(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":            "ref: refs/heads/main\n",
		"refs/heads/main": "aaa\n",
	})
	got := readGitState(repo)
	if !got.LastFetch.IsZero() {
		t.Errorf("no FETCH_HEAD and no remote refs must give a zero time, got %v", got.LastFetch)
	}
}

func TestReadGitState_LastFetchFromFetchHead(t *testing.T) {
	repo := writeGit(t, map[string]string{
		"HEAD":            "ref: refs/heads/main\n",
		"refs/heads/main": "aaa\n",
		"FETCH_HEAD":      "aaa\tbranch 'main' of github.com:x/y\n",
	})
	got := readGitState(repo)
	if got.LastFetch.IsZero() {
		t.Fatal("FETCH_HEAD present, LastFetch must be set")
	}
	if time.Since(got.LastFetch) > time.Minute {
		t.Errorf("LastFetch should be ~now, got %v", got.LastFetch)
	}
}

func TestReadGitState_NoGitDir(t *testing.T) {
	got := readGitState(t.TempDir())
	if !got.Detached || got.InSyncWithOrigin() {
		t.Errorf("a directory with no .git must not read as in sync: %+v", got)
	}
}
