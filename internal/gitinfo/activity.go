// Package gitinfo reads git activity of the scanned repos: branches and last
// commit times. It execs git lazily — never during the base scan.
package gitinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// staleAfter marks a repo inactive when its last commit is older than this.
const staleAfter = 180 * 24 * time.Hour

// Activity is the git state of one repo.
type Activity struct {
	Service       string    `json:"service"`
	CurrentBranch string    `json:"current_branch"`
	Branches      []Branch  `json:"branches"`
	LastCommit    time.Time `json:"last_commit,omitzero"`
	LastSubject   string    `json:"last_subject,omitempty"`
	Stale         bool      `json:"stale"` // no commit in 6 months
}

// Branch is one local branch with a purpose guessed from its name.
type Branch struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"` // ticket | release | feature | main
}

// Load reads the git activity of one repo.
func Load(repoPath, serviceName string) Activity {
	activity := Activity{Service: serviceName, CurrentBranch: currentBranch(repoPath)}

	for _, name := range localBranches(repoPath) {
		activity.Branches = append(activity.Branches, Branch{Name: name, Purpose: classifyBranch(name)})
	}

	out, err := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%ct\t%s").Output()
	if err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
		if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			activity.LastCommit = time.Unix(ts, 0).UTC()
			activity.Stale = time.Since(activity.LastCommit) > staleAfter
		}
		if len(parts) == 2 {
			activity.LastSubject = parts[1]
		}
	}
	return activity
}

// LoadAll reads activity for many repos concurrently.
func LoadAll(repos map[string]string) []Activity {
	type job struct{ service, path string }
	jobs := make([]job, 0, len(repos))
	for service, path := range repos {
		jobs = append(jobs, job{service, path})
	}

	results := make([]Activity, len(jobs))
	var group errgroup.Group
	group.SetLimit(16)
	for i, j := range jobs {
		group.Go(func() error {
			results[i] = Load(j.path, j.service)
			return nil
		})
	}
	_ = group.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].Service < results[j].Service })
	return results
}

func currentBranch(repoPath string) string {
	data, err := os.ReadFile(filepath.Join(repoPath, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	if name, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return name
	}
	return "(detached)"
}

// localBranches lists refs/heads, both loose and packed.
func localBranches(repoPath string) []string {
	seen := map[string]bool{}

	headsDir := filepath.Join(repoPath, ".git", "refs", "heads")
	_ = filepath.WalkDir(headsDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if rel, err := filepath.Rel(headsDir, path); err == nil {
				seen[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})

	if data, err := os.ReadFile(filepath.Join(repoPath, ".git", "packed-refs")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				if name, ok := strings.CutPrefix(fields[1], "refs/heads/"); ok {
					seen[name] = true
				}
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func classifyBranch(name string) string {
	lower := strings.ToLower(name)
	switch {
	case lower == "main" || lower == "master":
		return "main"
	case strings.HasPrefix(lower, "uf-") || strings.HasPrefix(lower, "draken-"):
		return "ticket"
	case isReleaseName(lower):
		return "release"
	default:
		return "feature"
	}
}

// isReleaseName matches "5.x", "12.0", "11-x" style names.
func isReleaseName(name string) bool {
	for _, r := range name {
		if (r < '0' || r > '9') && r != '.' && r != '-' && r != 'x' {
			return false
		}
	}
	return len(name) > 0
}
