package insight

import (
	"fmt"
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// maxPackEndpoints bounds the endpoint table in a context pack.
const maxPackEndpoints = 40

// BuildContextPack renders a compact markdown card of one service, sized for
// dropping into an agent's context.
func BuildContextPack(s *catalog.Service) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	if s.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", s.Description)
	}
	fmt.Fprintf(&b, "- Version %s · dept44 parent %s · repo %s\n", s.Version, s.Dept44Parent, s.RepoURL)
	fmt.Fprintf(&b, "- Local path: %s\n", s.Path)
	if s.API != nil {
		fmt.Fprintf(&b, "- API: %s %s (%d endpoints, spec at %s)\n", s.API.Title, s.API.Version, len(s.API.Endpoints), s.SpecPath)
	}
	fmt.Fprintf(&b, "- Database: %v · Scheduler: %v\n", s.UsesDatabase, s.UsesScheduler)

	if len(s.Integrations) > 0 {
		b.WriteString("\n## Calls\n")
		seen := map[string]bool{}
		for _, integration := range s.Integrations {
			target := integration.ResolvedTo
			if target == "" {
				target = integration.Name + " (external)"
			}
			if seen[target] {
				continue
			}
			seen[target] = true
			fmt.Fprintf(&b, "- %s\n", target)
		}
	}
	if len(s.ConsumedBy) > 0 {
		fmt.Fprintf(&b, "\n## Called by\n- %s\n", strings.Join(s.ConsumedBy, "\n- "))
	}

	if schema, err := LoadDBSchema(s.Path, s.Name, false); err == nil && len(schema.Tables) > 0 {
		names := make([]string, 0, len(schema.Tables))
		for _, table := range schema.Tables {
			names = append(names, table.Name)
		}
		fmt.Fprintf(&b, "\n## Database tables\n%s\n", strings.Join(names, ", "))
	}

	if surface := LoadConfigSurface(s); len(surface.EnvVars) > 0 {
		b.WriteString("\n## Env vars\n")
		for _, v := range surface.EnvVars {
			if v.HasDef {
				fmt.Fprintf(&b, "- %s (default: %s)\n", v.Name, v.Default)
			} else {
				fmt.Fprintf(&b, "- %s (required)\n", v.Name)
			}
		}
	}

	if s.API != nil && len(s.API.Endpoints) > 0 {
		b.WriteString("\n## Endpoints\n")
		for i, ep := range s.API.Endpoints {
			if i == maxPackEndpoints {
				fmt.Fprintf(&b, "- … %d more (use search_endpoints)\n", len(s.API.Endpoints)-maxPackEndpoints)
				break
			}
			fmt.Fprintf(&b, "- %s %s — %s\n", ep.Method, ep.Path, ep.Summary)
		}
	}

	return b.String()
}
