package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func unresolvedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unresolved",
		Short: "Integration names that resolved to no local repo (external systems or alias candidates)",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(c.External)
			}
			for _, name := range c.External {
				var users []string
				for i := range c.Services {
					s := &c.Services[i]
					for _, integration := range s.Integrations {
						if integration.Name == name && integration.ResolvedTo == "" {
							users = append(users, s.ShortName)
						}
					}
				}
				fmt.Printf("%-35s used by: %v\n", name, users)
			}
			fmt.Printf("\n%d unresolved names (add aliases in ~/.config/greve/config.yml if any are local repos)\n", len(c.External))
			return nil
		},
	}
}
