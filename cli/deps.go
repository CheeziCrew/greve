package cli

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func depsCmd() *cobra.Command {
	var versionPrefix string

	cmd := &cobra.Command{
		Use:   "deps <artifactId>",
		Short: "Which services use an artifact, at what version (dept44-service-parent works too)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			hits := c.DependencyVersions(args[0], versionPrefix)
			if jsonOutput {
				return printJSON(hits)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tVERSION")
			counts := map[string]int{}
			for _, h := range hits {
				fmt.Fprintf(w, "%s\t%s\n", h.Service, h.Version)
				counts[h.Version]++
			}
			if err := w.Flush(); err != nil {
				return err
			}

			versions := make([]string, 0, len(counts))
			for v := range counts {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			fmt.Printf("\n%d services:", len(hits))
			for _, v := range versions {
				fmt.Printf("  %s ×%d", v, counts[v])
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&versionPrefix, "version", "", "Only versions with this prefix")
	return cmd
}
