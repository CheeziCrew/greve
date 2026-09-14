package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/review"
)

type reviewIn struct {
	Service string   `json:"service" jsonschema:"service name (fuzzy)"`
	Changed bool     `json:"changed,omitempty" jsonschema:"only lint files changed vs base (committed since merge-base + working tree)"`
	Base    string   `json:"base,omitempty" jsonschema:"base ref for changed mode, default main"`
	Disable []string `json:"disable,omitempty" jsonschema:"rule IDs to skip for this run, unioned with .greve-review.yml (e.g. no-lombok for a service where Lombok is an accepted exception)"`
}

func (s *server) addReviewTools(impl *mcp.Server) {
	mcp.AddTool(impl, &mcp.Tool{
		Name:        "review_diff",
		Description: "Deterministic dept44 convention linter over a service's working tree, or only the files changed vs a base ref (changed=true). Returns rule violations with file, line, severity, message, suggested fix, and corpus_id (cross-reference into the standards corpus). Error-severity findings should block a merge; warnings advise. Catches lexical conventions the Maven build and SonarCloud don't: ternaries, missing {Resource}FailureTest, missing @CircuitBreaker on Feign/repositories, Lombok, org.zalando.problem imports, enums in api/model, @Scheduled vs @Dept44Scheduled, field injection, non-static-imported HttpStatus.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in reviewIn) (*mcp.CallToolResult, review.Result, error) {
		svc := s.current().Lookup(in.Service)
		if svc == nil {
			return nil, review.Result{}, fmt.Errorf("no service matching %q", in.Service)
		}
		base := in.Base
		if base == "" {
			base = "main"
		}
		cfg := review.LoadConfig(svc.Path)
		cfg.DisableRules(in.Disable)
		res, err := review.Run(svc.Path, svc.Name, review.Options{Changed: in.Changed, Base: base}, cfg)
		if err != nil {
			return nil, review.Result{}, err
		}
		return nil, res, nil
	})
}
