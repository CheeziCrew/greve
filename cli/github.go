package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/github"
)

func githubCmd() *cobra.Command {
	var refresh, all bool

	cmd := &cobra.Command{
		Use:   "github",
		Short: "Compare GitHub org repos against local clones (needs gh, cached 24h)",
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			overview, warn, err := github.Compare(root, loadConfig().Orgs, refresh)
			if err != nil {
				return err
			}
			if !all {
				overview.NotCloned = github.ServiceShaped(overview.NotCloned)
			}
			if warn != nil {
				fmt.Fprintf(os.Stderr, "warning: using stale cache: %v\n", warn)
			}
			if jsonOutput {
				return printJSON(overview)
			}

			if len(overview.NotCloned) > 0 {
				fmt.Printf("Org repos not cloned locally (%d):\n", len(overview.NotCloned))
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				for _, r := range overview.NotCloned {
					fmt.Fprintf(w, "  %s/%s\t%s\n", r.Org, r.Name, truncate(r.Description, 70))
				}
				if err := w.Flush(); err != nil {
					return err
				}
			}
			if len(overview.ArchivedCloned) > 0 {
				fmt.Printf("\nCloned locally but archived on GitHub (%d):\n", len(overview.ArchivedCloned))
				for _, r := range overview.ArchivedCloned {
					fmt.Printf("  %s/%s\n", r.Org, r.Name)
				}
			}
			fmt.Printf("\nlisting fetched %s\n", overview.FetchedAt.Format("2006-01-02 15:04 MST"))
			return nil
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force a fresh fetch from GitHub")
	cmd.Flags().BoolVar(&all, "all", false, "Include all org repos, not just service-shaped ones (api-*, pw-*, web-*)")
	return cmd
}
