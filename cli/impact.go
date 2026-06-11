package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/insight"
)

func impactCmd() *cobra.Command {
	var method, path, schema string

	cmd := &cobra.Command{
		Use:   "impact <service>",
		Short: "Which consumers would a change to an endpoint or schema affect",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			hits, err := insight.AnalyzeImpact(c, args[0], method, path, schema)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(hits)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "CONSUMER\tAFFECTED\tVENDORED\tSPEC\tDETAIL")
			affected := 0
			for _, h := range hits {
				if h.Affected == "yes" {
					affected++
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", h.Consumer, h.Affected, h.VendoredVersion, h.SpecFile, h.Detail)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d consumers checked, %d affected\n", len(hits), affected)
			return nil
		},
	}

	cmd.Flags().StringVar(&method, "method", "", "HTTP method of the changing endpoint")
	cmd.Flags().StringVar(&path, "path", "", "Path of the changing endpoint")
	cmd.Flags().StringVar(&schema, "schema", "", "Component schema name instead of an endpoint")
	return cmd
}
