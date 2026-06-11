package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/insight"
)

type patternsIn struct {
	Type string `json:"type" jsonschema:"component type: scheduler, feign, validator, apptest, mapper, resource, or entity"`
}

type patternsOut struct {
	Examples []insight.PatternExample `json:"examples"`
}

type packOut struct {
	Markdown string `json:"markdown"`
}

func (s *server) addAgentTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "test_coverage",
		Description: "Integration-test suites and WireMock scenarios of a service, plus which integrations the stub fixtures touch (heuristic: name match in fixture paths).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in serviceOnlyIn) (*mcp.CallToolResult, *insight.CoverageReport, error) {
		c := s.current()
		svc := c.Lookup(in.Service)
		if svc == nil {
			return nil, nil, fmt.Errorf("no service matching %q", in.Service)
		}
		return nil, insight.LoadCoverage(svc), nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "pattern_examples",
		Description: "Real production examples of a dept44 component type (scheduler, feign, validator, apptest, mapper, resource, entity) from the healthiest repos. Use as grounding when writing new components.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in patternsIn) (*mcp.CallToolResult, patternsOut, error) {
		examples := insight.FindPatternExamples(s.current(), in.Type)
		if examples == nil {
			return nil, patternsOut{}, fmt.Errorf("unknown pattern type %q (valid: %s)", in.Type, strings.Join(insight.PatternTypes, ", "))
		}
		return nil, patternsOut{Examples: examples}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "context_pack",
		Description: "Compact markdown card of one service (purpose, endpoints, integrations both directions, db tables, env vars, local path) sized for an agent's context. Prefer this over get_service when you want orientation rather than raw data.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in serviceOnlyIn) (*mcp.CallToolResult, packOut, error) {
		c := s.current()
		svc := c.Lookup(in.Service)
		if svc == nil {
			return nil, packOut{}, fmt.Errorf("no service matching %q", in.Service)
		}
		return nil, packOut{Markdown: insight.BuildContextPack(svc)}, nil
	})
}
