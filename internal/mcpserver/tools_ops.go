package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/insight"
)

type dbSchemaIn struct {
	Service string `json:"service" jsonschema:"service name (fuzzy)"`
	History bool   `json:"history,omitempty" jsonschema:"include the migration history list"`
}

type serviceOnlyIn struct {
	Service string `json:"service" jsonschema:"service name (fuzzy)"`
}

type optionalServiceIn struct {
	Service string `json:"service,omitempty" jsonschema:"one service, or empty for the whole fleet"`
}

type jobsOut struct {
	Jobs []insight.Job `json:"jobs"`
}

type resilienceOut struct {
	Edges []insight.ResilienceEdge `json:"edges"`
}

func (s *server) addOpsTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "db_schema",
		Description: "Database tables of a service (columns, FKs, indexes) reconstructed from its Flyway migrations. Regex-parsed DDL — exotic statements are skipped, not misread. Set history=true for the migration timeline.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in dbSchemaIn) (*mcp.CallToolResult, *insight.DBSchema, error) {
		c := s.current()
		svc := c.Lookup(in.Service)
		if svc == nil {
			return nil, nil, fmt.Errorf("no service matching %q", in.Service)
		}
		schema, err := insight.LoadDBSchema(svc.Path, svc.Name, in.History)
		if err != nil {
			return nil, nil, fmt.Errorf("%s has no Flyway migrations", svc.Name)
		}
		return nil, schema, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "config_surface",
		Description: "Env vars and ${VAR:default} placeholders a service needs to run — its deploy checklist, from the base application.yml.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in serviceOnlyIn) (*mcp.CallToolResult, *insight.ConfigSurface, error) {
		c := s.current()
		svc := c.Lookup(in.Service)
		if svc == nil {
			return nil, nil, fmt.Errorf("no service matching %q", in.Service)
		}
		return nil, insight.LoadConfigSurface(svc), nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "scheduler_jobs",
		Description: "Scheduled jobs with cron expressions, fleet-wide or per service. Sources: yml cron keys and @Dept44Scheduled annotations.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in optionalServiceIn) (*mcp.CallToolResult, jobsOut, error) {
		c := s.current()
		var out jobsOut
		if in.Service != "" {
			svc := c.Lookup(in.Service)
			if svc == nil {
				return nil, out, fmt.Errorf("no service matching %q", in.Service)
			}
			out.Jobs = insight.LoadJobs(svc)
			return nil, out, nil
		}
		for i := range c.Services {
			out.Jobs = append(out.Jobs, insight.LoadJobs(&c.Services[i])...)
		}
		return nil, out, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "resilience_report",
		Description: "Connect/read timeouts and circuit-breaker config per integration edge; flags edges with no explicit timeouts.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in optionalServiceIn) (*mcp.CallToolResult, resilienceOut, error) {
		c := s.current()
		var out resilienceOut
		if in.Service != "" {
			svc := c.Lookup(in.Service)
			if svc == nil {
				return nil, out, fmt.Errorf("no service matching %q", in.Service)
			}
			out.Edges = insight.LoadResilience(svc)
			return nil, out, nil
		}
		for i := range c.Services {
			out.Edges = append(out.Edges, insight.LoadResilience(&c.Services[i])...)
		}
		return nil, out, nil
	})
}
