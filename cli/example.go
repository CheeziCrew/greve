package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/insight"
)

func exampleCmd() *cobra.Command {
	var consumer string

	cmd := &cobra.Command{
		Use:   "example <provider>",
		Short: "Real Feign client + config from an existing consumer of a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			example, err := insight.FindUsageExample(c, args[0], consumer)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(example)
			}

			fmt.Printf("How %s calls %s:\n", example.Consumer, example.Provider)
			for _, excerpt := range example.Excerpts {
				fmt.Printf("\n── %s: %s ──\n%s\n", excerpt.Role, excerpt.Path, excerpt.Content)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&consumer, "consumer", "", "Use this consumer instead of the best match")
	return cmd
}
