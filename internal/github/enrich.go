// Package github adds optional enrichment from the GitHub API via the gh
// CLI. Everything here degrades gracefully: no gh, no network, no token —
// the rest of greve is unaffected.
package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// DefaultOrgs are the GitHub organizations hosting the service repos.
var DefaultOrgs = []string{"Sundsvallskommun", "Public-Service-as-a-Service"}

// Repo is one repository as seen on GitHub.
type Repo struct {
	Name          string    `json:"name"`
	Org           string    `json:"org"`
	Description   string    `json:"description,omitempty"`
	Archived      bool      `json:"archived"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	PushedAt      time.Time `json:"pushed_at,omitzero"`
}

// fetchOrg lists all repos of one org via gh. The --paginate flag emits one
// JSON array per page back to back, so decode in a loop.
func fetchOrg(org string) ([]Repo, error) {
	cmd := exec.Command("gh", "api", fmt.Sprintf("orgs/%s/repos", org), "--paginate")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api orgs/%s/repos: %w", org, err)
	}

	var repos []Repo
	dec := json.NewDecoder(bytesReader(out))
	for dec.More() {
		var page []struct {
			Name          string    `json:"name"`
			Description   string    `json:"description"`
			Archived      bool      `json:"archived"`
			DefaultBranch string    `json:"default_branch"`
			PushedAt      time.Time `json:"pushed_at"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decoding gh output for %s: %w", org, err)
		}
		for _, r := range page {
			repos = append(repos, Repo{
				Name:          r.Name,
				Org:           org,
				Description:   r.Description,
				Archived:      r.Archived,
				DefaultBranch: r.DefaultBranch,
				PushedAt:      r.PushedAt,
			})
		}
	}
	return repos, nil
}
