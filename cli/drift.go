package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// staleFetchAfter is when a clone's view of origin stops being trustworthy.
const staleFetchAfter = 7 * 24 * time.Hour

func driftCmd() *cobra.Command {
	var fetch, all bool

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Local clones that misrepresent their service (branch, sha, or parent version vs origin)",
		Long: `Reports where the local tree disagrees with origin.

greve reads the clones under the root, so a checkout parked on a feature branch
or simply left behind will otherwise report a version the service moved past
months ago. Parent versions are already resolved against origin/<default>; this
command shows the clones behind that correction, so they can be pulled.

Never fetches unless asked: --fetch is opt-in, because a catalogue command that
quietly does network I/O on every repo is one you stop running.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if fetch {
				root, err := resolveRoot()
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "fetching all remotes (--fetch)…")
				if err := fetchAll(root); err != nil {
					return err
				}
			}

			c, err := loadCatalog()
			if err != nil {
				return err
			}

			drifted := make([]catalog.Service, 0)
			for _, s := range c.Services {
				if material(s.Git) || (all && s.Git.Drifted()) {
					drifted = append(drifted, s)
				}
			}

			if jsonOutput {
				if err := printJSON(drifted); err != nil {
					return err
				}
			} else {
				printCloneDrift(c, drifted)
			}

			warnNotRepositories(c)
			if len(drifted) > 0 {
				return fmt.Errorf("%d of %d clones drifted from origin", len(drifted), len(c.Services))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fetch, "fetch", false, "Run 'git fetch --all --prune' in every repo first (network; never automatic)")
	cmd.Flags().BoolVar(&all, "all", false, "Also list clones that merely sit on another branch without misreporting anything")
	return cmd
}

// material reports whether this clone can mislead. A checkout parked on a
// feature branch is normal here and not itself a problem — the parent version
// was already resolved against origin. What matters is a worktree that
// disagrees with origin, or one we could not verify against origin at all.
func material(g catalog.GitState) bool {
	return g.WorktreeParent != "" || !g.OriginVerified
}

func printCloneDrift(c *catalog.Catalog, drifted []catalog.Service) {
	if len(drifted) == 0 {
		fmt.Printf("%d clones, all in sync with origin\n", len(c.Services))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tBRANCH\tPARENT (ORIGIN)\tCHECKED OUT\tLAST FETCH\tWHY")
	for _, s := range drifted {
		branch := s.Git.Branch
		if s.Git.Detached {
			branch = "(detached)"
		}
		checkedOut := "="
		if s.Git.WorktreeParent != "" {
			checkedOut = s.Git.WorktreeParent
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.Name, branch, s.Dept44Parent, checkedOut,
			fetchAge(s.Git.LastFetch), strings.Join(driftReasons(s.Git), ", "))
	}
	_ = w.Flush()
	fmt.Println()
	printFetchAge(c)
}

// printFetchAge reports fetch staleness once for the fleet. Per-repo it is
// noise — they are all fetched together or not at all — and a list where every
// row carries the same reason is a list nobody reads.
func printFetchAge(c *catalog.Catalog) {
	stale, oldest := 0, time.Time{}
	for _, s := range c.Services {
		if !staleFetch(s.Git.LastFetch) {
			continue
		}
		stale++
		if oldest.IsZero() || (!s.Git.LastFetch.IsZero() && s.Git.LastFetch.Before(oldest)) {
			oldest = s.Git.LastFetch
		}
	}
	if stale == 0 {
		return
	}
	fmt.Printf("%d clones last fetched over %dd ago (oldest %s) — 'greve drift --fetch' refreshes them\n",
		stale, int(staleFetchAfter.Hours()/24), fetchAge(oldest))
}

func driftReasons(g catalog.GitState) []string {
	var reasons []string
	if g.Detached {
		reasons = append(reasons, "detached HEAD")
	} else if g.Branch != g.DefaultBranch {
		reasons = append(reasons, "on "+g.Branch)
	} else if g.HeadSHA != g.OriginSHA {
		reasons = append(reasons, "behind/ahead of origin")
	}
	if g.WorktreeParent != "" {
		reasons = append(reasons, "stale parent in worktree")
	}
	if !g.OriginVerified {
		reasons = append(reasons, "unverified against origin")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "differs from origin")
	}
	return reasons
}

// staleFetch treats an unknown fetch time as stale. A clone that has never
// fetched must never read as up to date.
func staleFetch(t time.Time) bool {
	return t.IsZero() || time.Since(t) > staleFetchAfter
}

func fetchAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	days := int(time.Since(t).Hours() / 24)
	if days < 1 {
		return "today"
	}
	return fmt.Sprintf("%dd", days)
}

func fetchAll(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var group errgroup.Group
	group.SetLimit(8)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := root + string(os.PathSeparator) + entry.Name()
		if _, err := os.Stat(path + string(os.PathSeparator) + ".git"); err != nil {
			continue
		}
		group.Go(func() error {
			// Fetch only. Never pull, rebase, or touch a working tree: repos
			// here sit on feature branches and carry uncommitted work.
			out, err := exec.Command("git", "-C", path, "fetch", "--all", "--prune", "--quiet").CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  fetch failed: %s: %s\n", entry.Name(), strings.TrimSpace(string(out)))
			}
			return nil
		})
	}
	return group.Wait()
}

// warnDrift prints one stderr line when the catalogue is built on clones that
// disagree with origin. stderr keeps stdout pipeable and --json clean.
func warnDrift(c *catalog.Catalog) {
	drifted, staleParent := 0, 0
	for _, s := range c.Services {
		if material(s.Git) {
			drifted++
		}
		if s.Git.WorktreeParent != "" {
			staleParent++
		}
	}
	if drifted == 0 {
		return
	}
	msg := fmt.Sprintf("note: %d of %d clones could misrepresent their service", drifted, len(c.Services))
	if staleParent > 0 {
		msg += fmt.Sprintf(" (%d have a stale parent version checked out; reported versions are origin's)", staleParent)
	}
	fmt.Fprintln(os.Stderr, msg+" — run 'greve drift' for detail")
}
