package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/mining"
)

func mineReviewsCmd() *cobra.Command {
	var repoList []string
	var all bool
	var maxPRs int
	var resume, dryRun bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "mine-reviews",
		Short: "Mine human PR review comments across dept44 Java repos into a local corpus (Phase 2 of the review robot)",
		Long: "Pulls substantive PR review comments via gh GraphQL, filters bots/noise, and appends them " +
			"to a local NDJSON store for later distillation into the standards corpus by a Claude Workflow. " +
			"Mature, high-traffic repos are mined first. Resumable; deduplicated by comment id.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			repos, err := resolveMineRepos(repoList, all)
			if err != nil {
				return err
			}
			if len(repos) == 0 {
				return fmt.Errorf("no repos to mine")
			}

			store, err := mining.OpenStore(dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			fmt.Printf("mining %d repo(s) (resume=%v dry-run=%v) — %d comments already stored at %s\n",
				len(repos), resume, dryRun, store.Count(), store.Path())

			stats := mining.Mine(repos, store, mining.Options{
				MaxPRsPerRepo: maxPRs,
				Resume:        resume,
				DryRun:        dryRun,
			}, func(m string) { fmt.Println("  " + m) })

			if jsonOutput {
				return printJSON(stats)
			}
			kept := 0
			for _, s := range stats {
				kept += s.Kept
			}
			fmt.Printf("\ndone: %d substantive comments kept this run; corpus now holds %d.\n", kept, store.Count())
			fmt.Printf("next: distil %s into the standards-corpus override (Claude Workflow), then greve serves it.\n", store.Path())
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&repoList, "repos", nil, "explicit owner/name repos (default: local dept44 Java repos, mature first)")
	cmd.Flags().BoolVar(&all, "all", false, "mine every local api-service-*/pw-*/dept44 repo (default already does this, mature first)")
	cmd.Flags().IntVar(&maxPRs, "max-prs-per-repo", 0, "cap PRs scanned per repo (0 = all)")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume from saved per-repo progress")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "count only; write nothing")
	cmd.Flags().StringVar(&dbPath, "db", "", "override store path (default ~/<cache>/greve/reviews.ndjson)")
	return cmd
}

// resolveMineRepos returns the repos to mine: explicit --repos, or the local
// dept44 Java repos ordered mature-first (newest dept44 parent, then endpoints).
func resolveMineRepos(explicit []string, _ bool) ([]mining.Repo, error) {
	if len(explicit) > 0 {
		var repos []mining.Repo
		for _, s := range explicit {
			org, name := "Sundsvallskommun", s
			if i := strings.IndexByte(s, '/'); i >= 0 {
				org, name = s[:i], s[i+1:]
			}
			repos = append(repos, mining.Repo{Org: org, Name: name})
		}
		return repos, nil
	}

	c, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	svcs := make([]*catalog.Service, 0, len(c.Services))
	for i := range c.Services {
		if isDept44JavaRepo(c.Services[i].Name) {
			svcs = append(svcs, &c.Services[i])
		}
	}
	sort.Slice(svcs, func(i, j int) bool {
		if svcs[i].Dept44Parent != svcs[j].Dept44Parent {
			return svcs[i].Dept44Parent > svcs[j].Dept44Parent // newest parent first
		}
		return endpointsOf(svcs[i]) > endpointsOf(svcs[j])
	})

	var repos []mining.Repo
	for _, s := range svcs {
		org := s.Org
		if org == "" {
			org = "Sundsvallskommun"
		}
		repos = append(repos, mining.Repo{Org: org, Name: s.Name})
	}
	return repos, nil
}

func isDept44JavaRepo(name string) bool {
	return strings.HasPrefix(name, "api-service-") || strings.HasPrefix(name, "pw-") || name == "dept44"
}

func endpointsOf(s *catalog.Service) int {
	if s.API == nil {
		return 0
	}
	return len(s.API.Endpoints)
}
