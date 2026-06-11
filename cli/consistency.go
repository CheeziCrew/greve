package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/insight"
)

func consistencyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "consistency [service]",
		Short: "Integration drift: Feign clients vs yml config vs vendored specs vs pom",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}

			var targets []*catalog.Service
			if len(args) == 1 {
				s := c.Lookup(args[0])
				if s == nil {
					return fmt.Errorf("no service matching %q", args[0])
				}
				targets = []*catalog.Service{s}
			} else {
				for i := range c.Services {
					targets = append(targets, &c.Services[i])
				}
			}

			var reports []insight.ConsistencyReport
			for _, s := range targets {
				report := insight.CheckConsistency(c, s)
				if len(report.ClientWithoutConfig)+len(report.ConfigWithoutClient)+len(report.SpecWithoutPom) > 0 {
					reports = append(reports, report)
				}
			}
			if jsonOutput {
				return printJSON(reports)
			}

			for _, r := range reports {
				fmt.Printf("%s (%d feign clients)\n", r.Service, r.FeignClients)
				printDrift("  client without yml config:", r.ClientWithoutConfig)
				printDrift("  yml config without client:", r.ConfigWithoutClient)
				printDrift("  vendored spec not in pom: ", r.SpecWithoutPom)
			}
			fmt.Printf("\n%d services with drift (config-without-client may just mean WebClient instead of Feign)\n", len(reports))
			return nil
		},
	}
}

func printDrift(label string, items []string) {
	if len(items) > 0 {
		fmt.Printf("%s %s\n", label, strings.Join(items, ", "))
	}
}
