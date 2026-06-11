// Package cli builds the cobra command tree for greve.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

var (
	rootDir    string
	jsonOutput bool
)

// config is the optional ~/.config/greve/config.yml.
type config struct {
	Root    string            `yaml:"root"`
	Aliases map[string]string `yaml:"aliases"`
	Orgs    []string          `yaml:"orgs"`
}

// BuildCLI creates the cobra command tree.
func BuildCLI() *cobra.Command {
	root := &cobra.Command{
		Use:           "greve",
		Short:         "greve — dept44 API catalogue (CLI + MCP server)",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVar(&rootDir, "root", "", "Directory containing the service repos (default $GREVE_ROOT or ~/Code/scit)")
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output JSON instead of tables")

	root.AddCommand(servicesCmd())
	root.AddCommand(serviceCmd())
	root.AddCommand(graphCmd())
	root.AddCommand(endpointsCmd())
	root.AddCommand(depsCmd())
	root.AddCommand(unresolvedCmd())
	root.AddCommand(exportCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(githubCmd())
	// Impact pack
	root.AddCommand(schemaCmd())
	root.AddCommand(impactCmd())
	root.AddCommand(staleCmd())
	root.AddCommand(consistencyCmd())
	root.AddCommand(exampleCmd())
	// Ops pack
	root.AddCommand(dbCmd())
	root.AddCommand(configCmd())
	root.AddCommand(jobsCmd())
	root.AddCommand(resilienceCmd())
	// Agent pack
	root.AddCommand(coverageCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(packCmd())
	// Fleet pack
	root.AddCommand(activityCmd())
	root.AddCommand(searchConfigCmd())
	root.AddCommand(pathCmd())
	root.AddCommand(fleetCmd())

	return root
}

func loadConfig() config {
	var cfg config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "greve", "config.yml"))
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

// resolveRoot picks the scan root: --root flag > GREVE_ROOT > config > default.
func resolveRoot() (string, error) {
	cfg := loadConfig()
	root := rootDir
	if root == "" {
		root = os.Getenv("GREVE_ROOT")
	}
	if root == "" {
		root = cfg.Root
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, "Code", "scit")
	}
	if expanded, err := expandHome(root); err == nil {
		root = expanded
	}
	return root, nil
}

func expandHome(path string) (string, error) {
	if len(path) < 2 || path[:2] != "~/" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path, err
	}
	return filepath.Join(home, path[2:]), nil
}

// loadCatalog scans the root and builds the catalog. Shared by all commands.
func loadCatalog() (*catalog.Catalog, error) {
	root, err := resolveRoot()
	if err != nil {
		return nil, err
	}
	services, err := scan.Scan(root)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", root, err)
	}
	return catalog.Build(root, services, loadConfig().Aliases), nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
