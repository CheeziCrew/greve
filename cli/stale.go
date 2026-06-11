package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/insight"
)

func staleCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "stale [provider]",
		Short: "Vendored client specs that lag behind the provider's current API version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			provider := ""
			if len(args) == 1 {
				provider = args[0]
			}
			hits := insight.StaleClients(c, provider)
			if !all {
				stale := hits[:0]
				for _, h := range hits {
					if h.Stale {
						stale = append(stale, h)
					}
				}
				hits = stale
			}
			if jsonOutput {
				return printJSON(hits)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "CONSUMER\tPROVIDER\tVENDORED\tCURRENT")
			for _, h := range hits {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Consumer, h.Provider, h.VendoredVersion, h.CurrentVersion)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d stale client specs\n", len(hits))
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Include up-to-date client specs too")
	return cmd
}
