package github

import (
	"os"
	"strings"
	"time"
)

// Overview compares the GitHub org listing against the local clones.
type Overview struct {
	FetchedAt      time.Time `json:"fetched_at"`
	NotCloned      []Repo    `json:"not_cloned"`
	ArchivedCloned []Repo    `json:"archived_cloned"`
}

// ServiceShaped filters a repo list down to the naming patterns used by
// deployable services.
func ServiceShaped(repos []Repo) []Repo {
	var out []Repo
	for _, r := range repos {
		for _, prefix := range []string{"api-", "pw-", "web-", "dept44"} {
			if strings.HasPrefix(r.Name, prefix) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// Compare fetches (or reads from cache) the org repo listings and diffs them
// against the directories under root. The returned warning is non-nil when a
// stale cache had to be used because fetching failed.
func Compare(root string, orgs []string, refresh bool) (overview *Overview, warn, err error) {
	if len(orgs) == 0 {
		orgs = DefaultOrgs
	}
	cache, fetchErr := Load(orgs, refresh)
	if cache == nil {
		return nil, nil, fetchErr
	}

	local := map[string]bool{}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				local[e.Name()] = true
			}
		}
	}

	overview = &Overview{FetchedAt: cache.FetchedAt}
	for _, r := range cache.Repos {
		switch {
		case !local[r.Name] && !r.Archived:
			overview.NotCloned = append(overview.NotCloned, r)
		case local[r.Name] && r.Archived:
			overview.ArchivedCloned = append(overview.ArchivedCloned, r)
		}
	}
	return overview, fetchErr, nil
}
