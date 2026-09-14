package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/standards"
)

type conventionRulesIn struct {
	Category string `json:"category,omitempty" jsonschema:"filter by layer: resource|pojo|entity|repository|mapper|integration|scheduler|service|validation|apptest|general; empty for all"`
}

type conventionRulesOut struct {
	Rules []standards.Rule `json:"rules"`
	Count int              `json:"count"`
}

type standardsForFileIn struct {
	Path string `json:"path" jsonschema:"a Java file path; its component layer is inferred from the path"`
}

type standardsForFileOut struct {
	Layer string           `json:"layer"`
	Rules []standards.Rule `json:"rules"`
	Count int              `json:"count"`
}

func (s *server) addStandardsTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "convention_rules",
		Description: "The dept44 convention rulebook, distilled from CLAUDE.md, the pattern references, and (once mined) recurring PR review feedback. Filter by component category. Each rule carries a statement, rationale, good/bad examples, and whether a 'greve review' linter rule already enforces it (deterministic + linter_rule_id).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in conventionRulesIn) (*mcp.CallToolResult, conventionRulesOut, error) {
		c, err := standards.Load()
		if err != nil {
			return nil, conventionRulesOut{}, err
		}
		rules := c.ByCategory(in.Category)
		return nil, conventionRulesOut{Rules: rules, Count: len(rules)}, nil
	})

	mcp.AddTool(impl, &mcp.Tool{
		Name:        "standards_for_file",
		Description: "Given a Java file path, infer its dept44 component layer and return the convention rules that apply (its layer plus general rules). Use before reviewing or writing a file.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in standardsForFileIn) (*mcp.CallToolResult, standardsForFileOut, error) {
		c, err := standards.Load()
		if err != nil {
			return nil, standardsForFileOut{}, err
		}
		layer, rules := c.ForPath(in.Path)
		return nil, standardsForFileOut{Layer: layer, Rules: rules, Count: len(rules)}, nil
	})
}
