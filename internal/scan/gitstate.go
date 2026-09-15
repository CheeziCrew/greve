package scan

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// readGitState reads what a repo's .git says about its position relative to
// origin, without executing git and without touching the network.
//
// This runs during the base scan, which is pure filesystem and sub-100ms for
// the whole fleet. Anything needing an exec belongs in the verify pass.
func readGitState(repoPath string) catalog.GitState {
	gitDir := filepath.Join(repoPath, ".git")

	state := catalog.GitState{
		Branch:        currentBranch(gitDir),
		DefaultBranch: defaultBranch(gitDir),
	}
	state.Detached = state.Branch == ""

	if state.DefaultBranch == "" {
		state.DefaultBranch = "main"
	}
	state.LastFetch = lastFetch(gitDir)

	if !state.Detached {
		state.HeadSHA = resolveRef(gitDir, "refs/heads/"+state.Branch)
	}
	state.OriginSHA = resolveRef(gitDir, "refs/remotes/origin/"+state.DefaultBranch)

	return state
}

// currentBranch returns "" for a detached HEAD.
func currentBranch(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	name, _ := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: refs/heads/")
	if name == strings.TrimSpace(string(data)) {
		return "" // not a symbolic ref: detached
	}
	return name
}

// defaultBranch reads refs/remotes/origin/HEAD, which git writes at clone time.
func defaultBranch(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "refs", "remotes", "origin", "HEAD"))
	if err == nil {
		if name, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: refs/remotes/origin/"); ok {
			return name
		}
	}
	// origin/HEAD is a symbolic ref, so it never appears in packed-refs as a
	// sha line. Callers fall back to "main".
	return ""
}

// lastFetch is the mtime of FETCH_HEAD, falling back to the newest remote ref.
// A zero time means unknown — never "fresh". A clone that has never fetched
// must not read as up to date.
func lastFetch(gitDir string) time.Time {
	if info, err := os.Stat(filepath.Join(gitDir, "FETCH_HEAD")); err == nil {
		return info.ModTime().UTC()
	}
	var newest time.Time
	remotes := filepath.Join(gitDir, "refs", "remotes", "origin")
	entries, err := os.ReadDir(remotes)
	if err != nil {
		return time.Time{}
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return time.Time{}
	}
	return newest.UTC()
}

// resolveRef reads a ref as a loose file, falling back to packed-refs.
func resolveRef(gitDir, ref string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(ref)))
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	for _, line := range packedRefs(gitDir) {
		sha, name, ok := strings.Cut(line, " ")
		if ok && name == ref {
			return sha
		}
	}
	return ""
}

func packedRefs(gitDir string) []string {
	data, err := os.ReadFile(filepath.Join(gitDir, "packed-refs"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		out = append(out, line)
	}
	return out
}
