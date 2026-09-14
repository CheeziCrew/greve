package review

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ChangedFiles returns the set of repo-relative, forward-slash .java paths that
// differ from base: committed since the merge-base (base...HEAD), plus the
// working tree (staged, unstaged, and untracked). base defaults to "main". A
// missing base ref degrades gracefully to just the working-tree diff. Execs git
// the same way the gitinfo package does — never during the base scan.
func ChangedFiles(repoPath, base string) map[string]bool {
	if base == "" {
		base = "main"
	}
	set := map[string]bool{}
	add := func(out []byte) {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, ".java") {
				set[filepath.ToSlash(line)] = true
			}
		}
	}
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", repoPath}, args...)...).Output(); err == nil {
			add(out)
		}
	}
	run("diff", "--name-only", "--diff-filter=ACMR", base+"...HEAD")
	run("diff", "--name-only", "--diff-filter=ACMR", "HEAD") // staged + unstaged
	run("ls-files", "--others", "--exclude-standard")        // untracked
	return set
}
