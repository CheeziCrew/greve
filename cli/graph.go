package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func graphCmd() *cobra.Command {
	var direction string
	var depth int
	var dot bool

	cmd := &cobra.Command{
		Use:   "graph [service]",
		Short: "Show call edges around a service (or the whole graph)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
				if c.Lookup(name) == nil {
					return fmt.Errorf("no service matching %q", name)
				}
			}
			edges := c.Graph(name, direction, depth)
			if jsonOutput {
				return printJSON(edges)
			}
			if dot {
				fmt.Println("digraph services {")
				fmt.Println("  rankdir=LR;")
				for _, e := range edges {
					style := ""
					if !e.Resolved {
						style = ` [style=dashed color=gray]`
					}
					fmt.Printf("  %q -> %q%s;\n", e.From, e.To, style)
				}
				fmt.Println("}")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "FROM\tTO\tRESOLVED\tEVIDENCE")
			for _, e := range edges {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.From, e.To, yesNo(e.Resolved), strings.Join(e.Sources, ", "))
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&direction, "direction", "both", "Edge direction: in, out, or both")
	cmd.Flags().IntVar(&depth, "depth", 1, "How many hops to follow")
	cmd.Flags().BoolVar(&dot, "dot", false, "Emit Graphviz dot format")

	return cmd
}
