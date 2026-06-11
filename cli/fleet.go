package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/gitinfo"
	"github.com/CheeziCrew/greve/internal/insight"
)

func activityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activity [service]",
		Short: "Git activity: current branch, local branches, last commit, staleness",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			targets, err := pickServices(c, args)
			if err != nil {
				return err
			}
			repos := map[string]string{}
			for _, s := range targets {
				repos[s.Name] = s.Path
			}
			activities := gitinfo.LoadAll(repos)
			if jsonOutput {
				return printJSON(activities)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tBRANCH\tBRANCHES\tLAST COMMIT\tSTALE\tSUBJECT")
			for _, a := range activities {
				last := ""
				if !a.LastCommit.IsZero() {
					last = a.LastCommit.Format("2006-01-02")
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
					a.Service, a.CurrentBranch, len(a.Branches), last, yesNo(a.Stale), truncate(a.LastSubject, 50))
			}
			return w.Flush()
		},
	}
}

func searchConfigCmd() *cobra.Command {
	var keysOnly bool

	cmd := &cobra.Command{
		Use:   "search-config <query>",
		Short: "Grep all application*.yml across the fleet (dotted key paths + values)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			hits := insight.SearchConfig(c, args[0], keysOnly)
			if jsonOutput {
				return printJSON(hits)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tFILE\tKEY\tVALUE")
			for _, h := range hits {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Service, h.File, h.KeyPath, truncate(h.Value, 50))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d hits\n", len(hits))
			return nil
		},
	}

	cmd.Flags().BoolVar(&keysOnly, "keys", false, "Match key paths only, not values")
	return cmd
}

func pathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <from> <to>",
		Short: "Shortest call chain between two services",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			path := c.PathBetween(args[0], args[1])
			if jsonOutput {
				return printJSON(path)
			}
			if path == nil {
				fmt.Printf("no call chain from %s to %s within 5 hops\n", args[0], args[1])
				return nil
			}
			fmt.Println(strings.Join(path, " → "))
			return nil
		},
	}
}

func fleetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fleet",
		Short: "Landscape health overview",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			report := insight.BuildFleetReport(c)
			if jsonOutput {
				return printJSON(report)
			}

			fmt.Printf("%d services · %d with database · %d with scheduler\n\n", report.Services, report.WithDatabase, report.WithScheduler)

			versions := make([]string, 0, len(report.ParentVersions))
			for v := range report.ParentVersions {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			fmt.Print("dept44 parent: ")
			for _, v := range versions {
				fmt.Printf(" %s ×%d", v, report.ParentVersions[v])
			}
			fmt.Println()

			printFleetList("placeholder README description", report.PlaceholderReadme)
			printFleetList("no README description", report.MissingDescription)
			printFleetList("no CODEOWNERS", report.MissingOwners)
			printFleetList("no standard CI workflows", report.MissingCI)
			printFleetList("no OpenAPI spec found", report.MissingSpec)
			fmt.Printf("\nstale vendored client specs: %d (greve stale)\n", report.StaleClients)
			fmt.Printf("unresolved external systems: %d (greve unresolved)\n", len(report.UnresolvedExternal))
			return nil
		},
	}
}

func printFleetList(label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s (%d):\n  %s\n", label, len(items), strings.Join(items, ", "))
}
