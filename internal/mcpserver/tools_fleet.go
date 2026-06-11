package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/gitinfo"
	"github.com/CheeziCrew/greve/internal/insight"
)

type searchConfigIn struct {
	Query    string `json:"query" jsonschema:"substring matched against dotted key paths and values"`
	KeysOnly bool   `json:"keys_only,omitempty" jsonschema:"match key paths only"`
}

type searchConfigOut struct {
	Hits  []insight.ConfigHit `json:"hits"`
	Count int                 `json:"count"`
}

type pathIn struct {
	From string `json:"from" jsonschema:"calling service (fuzzy name)"`
	To   string `json:"to" jsonschema:"target service (fuzzy name)"`
}

type pathOut struct {
	Path  []string `json:"path"` // empty = no chain within 5 hops
	Found bool     `json:"found"`
}

type activityOut struct {
	Activities []gitinfo.Activity `json:"activities"`
}

func (s *server) addFleetTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "git_activity",
		Description: "Git activity per repo: current branch, local branches classified by purpose (ticket/release/feature), last commit, staleness flag (no commit in 6 months).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in optionalServiceIn) (*mcp.CallToolResult, activityOut, error) {
		c := s.current()
		repos := map[string]string{}
		if in.Service != "" {
			svc := c.Lookup(in.Service)
			if svc == nil {
				return nil, activityOut{}, fmt.Errorf("no service matching %q", in.Service)
			}
			repos[svc.Name] = svc.Path
		} else {
			for i := range c.Services {
				repos[c.Services[i].Name] = c.Services[i].Path
			}
		}
		return nil, activityOut{Activities: gitinfo.LoadAll(repos)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "search_config",
		Description: "Structured grep across every application*.yml in the fleet: returns service, file, dotted key path, value. Use for questions like 'who sets spring.flyway.enabled' or 'who configures X'.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchConfigIn) (*mcp.CallToolResult, searchConfigOut, error) {
		hits := insight.SearchConfig(s.current(), in.Query, in.KeysOnly)
		return nil, searchConfigOut{Hits: hits, Count: len(hits)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "path_between",
		Description: "Shortest service-to-service call chain over resolved integration edges (max 5 hops). Answers 'how does data get from A to B'.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, pathOut, error) {
		path := s.current().PathBetween(in.From, in.To)
		return nil, pathOut{Path: path, Found: path != nil}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "fleet_report",
		Description: "One-shot landscape health: dept44 parent version spread, repos missing README description/CODEOWNERS/standard CI/OpenAPI spec, placeholder READMEs, stale client count, unresolved externals.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, *insight.FleetReport, error) {
		return nil, insight.BuildFleetReport(s.current()), nil
	})
}
