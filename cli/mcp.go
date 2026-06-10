package cli

import (
	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/mcpserver"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the catalogue as an MCP server on stdio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			cfg := loadConfig()
			return mcpserver.Run(cmd.Context(), root, cfg.Aliases, cfg.Orgs)
		},
	}
}
