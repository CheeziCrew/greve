package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/insight"
)

func dbCmd() *cobra.Command {
	var history bool

	cmd := &cobra.Command{
		Use:   "db <service>",
		Short: "Database tables from Flyway migrations (regex-parsed DDL)",
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
			schema, err := insight.LoadDBSchema(s.Path, s.Name, history)
			if err != nil {
				return fmt.Errorf("%s has no Flyway migrations: %w", s.Name, err)
			}
			if jsonOutput {
				return printJSON(schema)
			}

			for _, t := range schema.Tables {
				fmt.Printf("%s  (since V%s)\n", t.Name, t.CreatedIn)
				for _, col := range t.Columns {
					fmt.Printf("    %-30s %s\n", col.Name, col.Type)
				}
				if len(t.ForeignKeys) > 0 {
					fmt.Printf("    FK: %s\n", strings.Join(t.ForeignKeys, ", "))
				}
				if len(t.Indexes) > 0 {
					fmt.Printf("    IX: %s\n", strings.Join(t.Indexes, ", "))
				}
			}
			fmt.Printf("\n%d tables\n", len(schema.Tables))
			if history {
				fmt.Println("\nMigrations:")
				for _, m := range schema.Migrations {
					fmt.Printf("  V%-8s %-45s [%s]\n", m.Version, m.Description, strings.Join(m.Tables, ", "))
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&history, "history", false, "Also list the migration history")
	return cmd
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config <service>",
		Short: "Env vars and placeholders a service needs (the deploy checklist)",
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
			surface := insight.LoadConfigSurface(s)
			if jsonOutput {
				return printJSON(surface)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ENV VAR\tDEFAULT\tUSED AT")
			for _, v := range surface.EnvVars {
				def := v.Default
				if !v.HasDef {
					def = "(required)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, def, v.KeyPath)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d placeholders in base application.yml\n", len(surface.EnvVars))
			return nil
		},
	}
}

func jobsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "jobs [service]",
		Short: "Scheduled jobs (yml cron keys + @Dept44Scheduled annotations)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			targets, err := pickServices(c, args)
			if err != nil {
				return err
			}

			var jobs []insight.Job
			for _, s := range targets {
				jobs = append(jobs, insight.LoadJobs(s)...)
			}
			if jsonOutput {
				return printJSON(jobs)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tJOB\tCRON")
			for _, j := range jobs {
				fmt.Fprintf(w, "%s\t%s\t%s\n", j.Service, j.Name, j.Cron)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d jobs\n", len(jobs))
			return nil
		},
	}
}

func resilienceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resilience [service]",
		Short: "Timeouts and circuit breakers per integration edge",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			targets, err := pickServices(c, args)
			if err != nil {
				return err
			}

			var edges []insight.ResilienceEdge
			for _, s := range targets {
				edges = append(edges, insight.LoadResilience(s)...)
			}
			if jsonOutput {
				return printJSON(edges)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tINTEGRATION\tCONNECT\tREAD\tBREAKER")
			missing := 0
			for _, e := range edges {
				if e.MissingTimeout {
					missing++
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.Service, e.Integration,
					orDash(e.ConnectTimeout), orDash(e.ReadTimeout), yesNo(e.CircuitBreaker))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("\n%d edges, %d without explicit timeouts\n", len(edges), missing)
			return nil
		},
	}
}

// pickServices resolves [name] args to one service or all services.
func pickServices(c *catalog.Catalog, args []string) ([]*catalog.Service, error) {
	if len(args) == 1 {
		s := c.Lookup(args[0])
		if s == nil {
			return nil, fmt.Errorf("no service matching %q", args[0])
		}
		return []*catalog.Service{s}, nil
	}
	var all []*catalog.Service
	for i := range c.Services {
		all = append(all, &c.Services[i])
	}
	return all, nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
