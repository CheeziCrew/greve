package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/insight"
	"github.com/CheeziCrew/greve/internal/scan"
)

type endpointSchemaIn struct {
	Service     string `json:"service" jsonschema:"service name (fuzzy)"`
	Method      string `json:"method,omitempty" jsonschema:"HTTP method, required together with path"`
	Path        string `json:"path,omitempty" jsonschema:"endpoint path exactly as in the spec, e.g. /{municipalityId}/sms"`
	OperationID string `json:"operation_id,omitempty" jsonschema:"alternative to method+path"`
}

type impactIn struct {
	Service string `json:"service" jsonschema:"the provider service whose API is changing"`
	Method  string `json:"method,omitempty" jsonschema:"HTTP method of the changing endpoint"`
	Path    string `json:"path,omitempty" jsonschema:"path of the changing endpoint"`
	Schema  string `json:"schema,omitempty" jsonschema:"component schema name instead of an endpoint"`
}

type impactOut struct {
	Hits []insight.ImpactHit `json:"consumers"`
}

type staleIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"limit to one provider service"`
	IncludeOk bool   `json:"include_ok,omitempty" jsonschema:"also return up-to-date client specs"`
}

type staleOut struct {
	Clients []insight.StaleHit `json:"clients"`
	Count   int                `json:"count"`
}

type consistencyIn struct {
	Service string `json:"service,omitempty" jsonschema:"one service, or empty for all services with drift"`
}

type consistencyOut struct {
	Reports []insight.ConsistencyReport `json:"reports"`
}

type exampleIn struct {
	Provider string `json:"provider" jsonschema:"the service you want to call"`
	Consumer string `json:"consumer,omitempty" jsonschema:"use this consumer instead of the best match"`
}

func (s *server) addImpactTools(impl *mcp.Server) {
	// Out is 'any' on purpose: OperationSchema carries free-form JSON
	// (request_body, responses), which schema inference turns into boolean
	// subschemas that MCP clients reject. With Out=any the SDK omits the
	// output schema and just returns the structured content.
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "endpoint_schema",
		Description: "Full request/response schema for one endpoint of a service, with $refs resolved inline. Use before constructing a call to the service. Select by method+path or operation_id.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in endpointSchemaIn) (*mcp.CallToolResult, any, error) {
		c := s.current()
		svc := c.Lookup(in.Service)
		if svc == nil {
			return nil, nil, fmt.Errorf("no service matching %q", in.Service)
		}
		if svc.SpecPath == "" {
			return nil, nil, fmt.Errorf("%s has no OpenAPI spec on disk", svc.Name)
		}
		op, err := scan.LoadOperationSchema(
			filepath.Join(svc.Path, filepath.FromSlash(svc.SpecPath)),
			strings.ToUpper(in.Method), in.Path, in.OperationID)
		if err != nil {
			return nil, nil, err
		}
		op.Service = svc.Name
		return nil, op, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "impact_analysis",
		Description: "Before changing an endpoint or schema of a service: which consumers' vendored client specs contain it (affected=yes), which don't, and which integrate config-only so impact is unknown.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in impactIn) (*mcp.CallToolResult, impactOut, error) {
		hits, err := insight.AnalyzeImpact(s.current(), in.Service, in.Method, in.Path, in.Schema)
		if err != nil {
			return nil, impactOut{}, err
		}
		return nil, impactOut{Hits: hits}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "stale_clients",
		Description: "Vendored client specs (integrations/*.yaml) whose version lags the provider's current API version. Org-wide outdated-integration report.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in staleIn) (*mcp.CallToolResult, staleOut, error) {
		hits := insight.StaleClients(s.current(), in.Provider)
		if !in.IncludeOk {
			stale := hits[:0]
			for _, h := range hits {
				if h.Stale {
					stale = append(stale, h)
				}
			}
			hits = stale
		}
		return nil, staleOut{Clients: hits, Count: len(hits)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "integration_consistency",
		Description: "Integration drift per service: Feign clients without yml config, yml config without a Feign client (may be WebClient — heuristic), vendored specs not wired into the pom.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in consistencyIn) (*mcp.CallToolResult, consistencyOut, error) {
		c := s.current()
		var out consistencyOut
		if in.Service != "" {
			svc := c.Lookup(in.Service)
			if svc == nil {
				return nil, out, fmt.Errorf("no service matching %q", in.Service)
			}
			out.Reports = append(out.Reports, insight.CheckConsistency(c, svc))
			return nil, out, nil
		}
		for i := range c.Services {
			report := insight.CheckConsistency(c, &c.Services[i])
			if len(report.ClientWithoutConfig)+len(report.ConfigWithoutClient)+len(report.SpecWithoutPom) > 0 {
				out.Reports = append(out.Reports, report)
			}
		}
		return nil, out, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "usage_examples",
		Description: "How to integrate with a service, shown via a real consumer: its Feign client interface, properties/configuration classes, and application.yml block. The fastest correct way to write a new integration.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in exampleIn) (*mcp.CallToolResult, *insight.UsageExample, error) {
		example, err := insight.FindUsageExample(s.current(), in.Provider, in.Consumer)
		if err != nil {
			return nil, nil, err
		}
		return nil, example, nil
	})
}
