package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/github"
)

type listServicesIn struct {
	Query        string `json:"query,omitempty" jsonschema:"substring match on name, description, or API title"`
	UsesArtifact string `json:"uses_artifact,omitempty" jsonschema:"only services with a declared dependency on this Maven artifactId"`
	HasDatabase  bool   `json:"has_database,omitempty" jsonschema:"only services using a database"`
	HasScheduler bool   `json:"has_scheduler,omitempty" jsonschema:"only services with scheduled jobs"`
	Org          string `json:"org,omitempty" jsonschema:"only services in this GitHub org"`
}

type serviceRow struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Dept44Parent string `json:"dept44_parent"`
	APITitle     string `json:"api_title,omitempty"`
	Endpoints    int    `json:"endpoints"`
	Integrations int    `json:"integrations"`
	Description  string `json:"description,omitempty"`
}

type listServicesOut struct {
	Services []serviceRow `json:"services"`
	Count    int          `json:"count"`
}

type getServiceIn struct {
	Name string `json:"name" jsonschema:"service name, fuzzy: citizen / api-service-citizen / artifactId all work"`
}

type serviceGraphIn struct {
	Name      string `json:"name" jsonschema:"service name (fuzzy)"`
	Direction string `json:"direction,omitempty" jsonschema:"in (callers), out (callees), or both (default)"`
	Depth     int    `json:"depth,omitempty" jsonschema:"hops to follow, default 1"`
}

type serviceGraphOut struct {
	Edges []catalog.Edge `json:"edges"`
}

type searchEndpointsIn struct {
	Query  string `json:"query" jsonschema:"matched case-insensitively against path, operationId, and summary"`
	Method string `json:"method,omitempty" jsonschema:"restrict to one HTTP method, e.g. POST"`
}

type searchEndpointsOut struct {
	Endpoints []catalog.EndpointHit `json:"endpoints"`
	Count     int                   `json:"count"`
}

type dependencyVersionsIn struct {
	Artifact      string `json:"artifact" jsonschema:"Maven artifactId; dept44-service-parent is special-cased to the parent POM version"`
	VersionPrefix string `json:"version_prefix,omitempty" jsonschema:"only versions starting with this prefix, e.g. 8.0"`
}

type dependencyVersionsOut struct {
	Services []catalog.VersionHit `json:"services"`
	Count    int                  `json:"count"`
}

type githubOverviewIn struct {
	Refresh bool `json:"refresh,omitempty" jsonschema:"force a fresh fetch from GitHub instead of the 24h cache"`
	All     bool `json:"all,omitempty" jsonschema:"include all org repos, not just service-shaped ones (api-*, pw-*, web-*)"`
}

type refreshOut struct {
	Services      int   `json:"services"`
	ExternalCount int   `json:"external_count"`
	DurationMs    int64 `json:"duration_ms"`
}

func (s *server) addTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "list_services",
		Description: "List dept44 microservices in the catalogue, optionally filtered by free text, dependency, database/scheduler usage, or GitHub org.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listServicesIn) (*mcp.CallToolResult, listServicesOut, error) {
		services := s.current().FilterServices(catalog.Filter{
			Query:        in.Query,
			UsesArtifact: in.UsesArtifact,
			HasDatabase:  in.HasDatabase,
			HasScheduler: in.HasScheduler,
			Org:          in.Org,
		})
		out := listServicesOut{Count: len(services), Services: make([]serviceRow, 0, len(services))}
		for _, svc := range services {
			row := serviceRow{
				Name:         svc.Name,
				Version:      svc.Version,
				Dept44Parent: svc.Dept44Parent,
				Integrations: len(svc.Integrations),
				Description:  svc.Description,
			}
			if svc.API != nil {
				row.APITitle = svc.API.Title
				row.Endpoints = len(svc.API.Endpoints)
			}
			out.Services = append(out.Services, row)
		}
		return nil, out, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "get_service",
		Description: "Everything known about one service: description, versions, API endpoints, integrations (who it calls), consumers (who calls it), database and scheduler usage.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getServiceIn) (*mcp.CallToolResult, *catalog.Service, error) {
		svc := s.current().Lookup(in.Name)
		if svc == nil {
			return nil, nil, fmt.Errorf("no service matching %q; use list_services to see available names", in.Name)
		}
		return nil, svc, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "service_graph",
		Description: "Call-graph edges around a service: who it calls (out), who calls it (in), or both. Unresolved targets are external systems.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in serviceGraphIn) (*mcp.CallToolResult, serviceGraphOut, error) {
		c := s.current()
		if c.Lookup(in.Name) == nil {
			return nil, serviceGraphOut{}, fmt.Errorf("no service matching %q", in.Name)
		}
		direction := in.Direction
		if direction == "" {
			direction = "both"
		}
		return nil, serviceGraphOut{Edges: c.Graph(in.Name, direction, in.Depth)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "search_endpoints",
		Description: "Find which service exposes an endpoint. Matches the query against path, operationId, and summary across all service OpenAPI specs.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchEndpointsIn) (*mcp.CallToolResult, searchEndpointsOut, error) {
		hits := s.current().SearchEndpoints(in.Query, in.Method)
		return nil, searchEndpointsOut{Endpoints: hits, Count: len(hits)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "dependency_versions",
		Description: "Which services depend on a Maven artifact and at what version. Use artifact dept44-service-parent to see the dept44 framework version per service (e.g. who is still on 8.0.5).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in dependencyVersionsIn) (*mcp.CallToolResult, dependencyVersionsOut, error) {
		hits := s.current().DependencyVersions(in.Artifact, in.VersionPrefix)
		return nil, dependencyVersionsOut{Services: hits, Count: len(hits)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "github_overview",
		Description: "Compare the GitHub org repo listings against local clones: repos not cloned locally and cloned repos that are archived upstream. Needs the gh CLI; results are cached 24h.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in githubOverviewIn) (*mcp.CallToolResult, *github.Overview, error) {
		overview, _, err := github.Compare(s.root, s.orgs, in.Refresh)
		if err != nil {
			return nil, nil, err
		}
		if !in.All {
			overview.NotCloned = github.ServiceShaped(overview.NotCloned)
		}
		return nil, overview, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "refresh_catalog",
		Description: "Rescan the repo tree and rebuild the catalogue. Use after pulling repos or changing branches.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, refreshOut, error) {
		start := time.Now()
		c, err := s.rescan()
		if err != nil {
			return nil, refreshOut{}, err
		}
		return nil, refreshOut{
			Services:      len(c.Services),
			ExternalCount: len(c.External),
			DurationMs:    time.Since(start).Milliseconds(),
		}, nil
	})
}
