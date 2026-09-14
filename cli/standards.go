package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/standards"
)

func standardsCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "standards [category]",
		Short: "Print the dept44 convention rulebook (optionally for one category or file)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := standards.Load()
			if err != nil {
				return err
			}

			var rules []standards.Rule
			var header string
			switch {
			case file != "":
				layer, r := c.ForPath(file)
				rules, header = r, fmt.Sprintf("%s (layer: %s)", file, layer)
			case len(args) == 1:
				rules, header = c.ByCategory(args[0]), args[0]
			default:
				rules, header = c.Rules, "all categories"
			}

			if jsonOutput {
				return printJSON(rules)
			}

			fmt.Printf("dept44 conventions — %s (%d rules)\n\n", header, len(rules))
			for _, r := range rules {
				mark := " "
				if r.Deterministic {
					mark = "✓"
				}
				fmt.Printf("[%s] %-30s (%s)\n      %s\n", mark, r.ID, r.Category, r.Statement)
			}
			fmt.Print("\n✓ = enforced by 'greve review'\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Show rules applicable to this file path")
	return cmd
}
