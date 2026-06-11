package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
)

func exportCmd() *cobra.Command {
	var format, out string
	var timestamp bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Emit the full catalogue as a CI-friendly artifact (json or markdown)",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			// Deterministic by default: same tree, byte-identical output.
			if !timestamp {
				c.GeneratedAt = time.Time{}
			}

			var data []byte
			switch format {
			case "json":
				data, err = json.MarshalIndent(c, "", "  ")
				if err != nil {
					return err
				}
				data = append(data, '\n')
			case "markdown", "md":
				data = []byte(renderMarkdown(c))
			default:
				return fmt.Errorf("unknown format %q (json or markdown)", format)
			}

			if out == "" || out == "-" {
				_, err = os.Stdout.Write(data)
				return err
			}
			return os.WriteFile(out, data, 0o644)
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or markdown")
	cmd.Flags().StringVar(&out, "out", "", "Write to file instead of stdout")
	cmd.Flags().BoolVar(&timestamp, "timestamp", false, "Include the generation timestamp (breaks determinism)")

	return cmd
}

func renderMarkdown(c *catalog.Catalog) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# API catalogue\n\n%d services scanned from `%s`.\n\n", len(c.Services), c.Root)

	for i := range c.Services {
		s := &c.Services[i]
		fmt.Fprintf(&b, "## %s\n\n", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Description)
		}
		fmt.Fprintf(&b, "- **Version:** %s (dept44 parent %s)\n", s.Version, s.Dept44Parent)
		if s.RepoURL != "" {
			fmt.Fprintf(&b, "- **Repo:** %s\n", s.RepoURL)
		}
		fmt.Fprintf(&b, "- **Database:** %s · **Scheduler:** %s\n", databaseLabel(s), yesNo(s.UsesScheduler))

		if len(s.Integrations) > 0 {
			var calls []string
			seen := map[string]bool{}
			for _, integration := range s.Integrations {
				target := integration.ResolvedTo
				if target == "" {
					target = integration.Name + " (external)"
				}
				if !seen[target] {
					seen[target] = true
					calls = append(calls, target)
				}
			}
			fmt.Fprintf(&b, "- **Calls:** %s\n", strings.Join(calls, ", "))
		}
		if len(s.ConsumedBy) > 0 {
			fmt.Fprintf(&b, "- **Called by:** %s\n", strings.Join(s.ConsumedBy, ", "))
		}

		if s.API != nil && len(s.API.Endpoints) > 0 {
			fmt.Fprintf(&b, "\n| Method | Path | Summary |\n|---|---|---|\n")
			for _, ep := range s.API.Endpoints {
				fmt.Fprintf(&b, "| %s | `%s` | %s |\n", ep.Method, ep.Path, ep.Summary)
			}
		}
		b.WriteString("\n")
	}

	if len(c.External) > 0 {
		fmt.Fprintf(&b, "## External systems\n\nIntegration targets with no local repo: %s\n", strings.Join(c.External, ", "))
	}
	return b.String()
}
