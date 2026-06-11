package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/insight"
)

func coverageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "coverage <service>",
		Short: "Integration-test suites, scenarios, and which integrations the stubs touch",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			s := c.Lookup(args[0])
			if s == nil {
				return fmt.Errorf("no service matching %q", args[0])
			}
			report := insight.LoadCoverage(s)
			if jsonOutput {
				return printJSON(report)
			}

			for _, suite := range report.Suites {
				fmt.Printf("%s (%d scenarios)\n", suite.Name, len(suite.Scenarios))
				for _, scenario := range suite.Scenarios {
					fmt.Printf("    %s\n", scenario)
				}
			}
			fmt.Printf("\ncovered integrations:   %s\n", strings.Join(report.CoveredIntegrations, ", "))
			fmt.Printf("uncovered integrations: %s\n", strings.Join(report.UncoveredIntegrations, ", "))
			fmt.Println("(heuristic: integration name appearing in stub fixture paths)")
			return nil
		},
	}
}

func patternsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "patterns <type>",
		Short: "Real in-org examples of a component type: " + strings.Join(insight.PatternTypes, ", "),
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			examples := insight.FindPatternExamples(c, args[0])
			if examples == nil {
				return fmt.Errorf("unknown pattern type %q (valid: %s)", args[0], strings.Join(insight.PatternTypes, ", "))
			}
			if jsonOutput {
				return printJSON(examples)
			}
			for _, example := range examples {
				fmt.Printf("%s\n    %s/%s\n    %s\n", example.Service, example.Service, example.File, example.Context)
			}
			return nil
		},
	}
}

func packCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pack <service>",
		Short: "Compact markdown context card of a service, sized for an agent's context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			s := c.Lookup(args[0])
			if s == nil {
				return fmt.Errorf("no service matching %q", args[0])
			}
			fmt.Print(insight.BuildContextPack(s))
			return nil
		},
	}
}
