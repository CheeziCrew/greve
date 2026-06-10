package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
)

func servicesCmd() *cobra.Command {
	var filter catalog.Filter

	cmd := &cobra.Command{
		Use:   "services",
		Short: "List services, optionally filtered",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			services := c.FilterServices(filter)
			if jsonOutput {
				return printJSON(services)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tPARENT\tDB\tSCHED\tENDPOINTS\tINTEGRATIONS\tDESCRIPTION")
			for _, s := range services {
				endpoints := 0
				if s.API != nil {
					endpoints = len(s.API.Endpoints)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
					s.Name, s.Version, s.Dept44Parent,
					yesNo(s.UsesDatabase), yesNo(s.UsesScheduler),
					endpoints, len(s.Integrations),
					truncate(s.Description, 60))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d services\n", len(services))
			return nil
		},
	}

	cmd.Flags().StringVar(&filter.Query, "query", "", "Substring match on name, description, API title")
	cmd.Flags().StringVar(&filter.UsesArtifact, "uses", "", "Only services depending on this artifactId")
	cmd.Flags().BoolVar(&filter.HasDatabase, "db", false, "Only services using a database")
	cmd.Flags().BoolVar(&filter.HasScheduler, "scheduler", false, "Only services with scheduled jobs")
	cmd.Flags().StringVar(&filter.Org, "org", "", "Only services in this GitHub org")

	return cmd
}

func serviceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "service <name>",
		Short: "Show everything known about one service",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			s := c.Lookup(args[0])
			if s == nil {
				return fmt.Errorf("no service matching %q (try 'greve services')", args[0])
			}
			if jsonOutput {
				return printJSON(s)
			}

			fmt.Printf("%s  (%s %s)\n", s.Name, s.ArtifactID, s.Version)
			if s.Description != "" {
				fmt.Printf("  %s\n", s.Description)
			}
			fmt.Printf("\n  dept44 parent: %s\n", s.Dept44Parent)
			if s.RepoURL != "" {
				fmt.Printf("  repo:          %s\n", s.RepoURL)
			}
			if s.API != nil {
				fmt.Printf("  api:           %s %s (%d endpoints, spec: %s)\n", s.API.Title, s.API.Version, len(s.API.Endpoints), s.SpecPath)
			}
			fmt.Printf("  database:      %s\n", databaseLabel(s))
			fmt.Printf("  scheduler:     %s\n", yesNo(s.UsesScheduler))

			if len(s.Integrations) > 0 {
				fmt.Println("\n  calls:")
				for _, i := range s.Integrations {
					target := i.ResolvedTo
					if target == "" {
						target = i.Name + " (external)"
					}
					fmt.Printf("    → %-40s [%s]\n", target, strings.Join(i.Sources, ", "))
				}
			}
			if len(s.ConsumedBy) > 0 {
				fmt.Println("\n  called by:")
				for _, caller := range s.ConsumedBy {
					fmt.Printf("    ← %s\n", caller)
				}
			}
			if s.API != nil && len(s.API.Endpoints) > 0 {
				fmt.Println("\n  endpoints:")
				for _, ep := range s.API.Endpoints {
					fmt.Printf("    %-7s %-60s %s\n", ep.Method, ep.Path, ep.Summary)
				}
			}
			return nil
		},
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func databaseLabel(s *catalog.Service) string {
	if !s.UsesDatabase {
		return "no"
	}
	if s.DatabaseKind != "" {
		return s.DatabaseKind
	}
	return "yes"
}
