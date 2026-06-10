package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func endpointsCmd() *cobra.Command {
	var method string

	cmd := &cobra.Command{
		Use:   "endpoints <query>",
		Short: "Find which service exposes an endpoint (matches path, operationId, summary)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			hits := c.SearchEndpoints(args[0], method)
			if jsonOutput {
				return printJSON(hits)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tMETHOD\tPATH\tSUMMARY")
			for _, h := range hits {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Service, h.Method, h.Path, truncate(h.Summary, 60))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d endpoints\n", len(hits))
			return nil
		},
	}

	cmd.Flags().StringVar(&method, "method", "", "Restrict to one HTTP method")
	return cmd
}
