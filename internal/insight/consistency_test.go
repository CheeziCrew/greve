package insight

import (
	"testing"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// provider with a small served surface, built through catalog.Build so the
// config-key resolution (ResolveIntegration) works exactly as in production.
func reachCatalog() *catalog.Catalog {
	prov := catalog.Service{
		Name:      "api-service-prov",
		ShortName: "prov",
		API: &catalog.APIInfo{
			Version: "1.0",
			Endpoints: []catalog.Endpoint{
				{Method: "POST", Path: "/{municipalityId}/errands/calculation/prepare"},
				{Method: "POST", Path: "/{municipalityId}/messages"},
				{Method: "GET", Path: "/{municipalityId}/{type}/partyId"},
			},
		},
	}
	return catalog.Build("/root", []catalog.Service{prov}, nil)
}

func client(calls ...scan.ClientCall) scan.FeignClient {
	return scan.FeignClient{Package: "prov", ConfigKey: "prov", Calls: calls}
}

func paths(hits []UnreachableCall) map[string]bool {
	m := map[string]bool{}
	for _, h := range hits {
		m[h.Path] = true
	}
	return m
}

func TestUnreachableFlagsRenamedRoute(t *testing.T) {
	c := reachCatalog()
	// One good route (so base paths align) + one renamed route.
	hits := unreachableCalls(c, []scan.FeignClient{client(
		scan.ClientCall{Method: "POST", Path: "/{municipalityId}/errands/calculation/prepare"},
		scan.ClientCall{Method: "POST", Path: "/{municipalityId}/errands/normberakning/prepare"},
	)})
	got := paths(hits)
	if got["/{municipalityId}/errands/calculation/prepare"] {
		t.Error("served route flagged as unreachable")
	}
	if !got["/{municipalityId}/errands/normberakning/prepare"] {
		t.Error("renamed route not flagged — the careM↔operaton drift would slip through")
	}
}

func TestUnreachableSuppressesWholeClientOffset(t *testing.T) {
	c := reachCatalog()
	// No route matches the provider (e.g. a context-path offset) — suppress all
	// rather than flag every route as broken.
	hits := unreachableCalls(c, []scan.FeignClient{client(
		scan.ClientCall{Method: "POST", Path: "/api/v3/{municipalityId}/errands/calculation/prepare"},
	)})
	if len(hits) != 0 {
		t.Errorf("whole-client base-path offset should be suppressed, got %d hits", len(hits))
	}
}

func TestUnreachableProviderVariableIsWildcard(t *testing.T) {
	c := reachCatalog()
	// Client hardcodes ENTERPRISE where the provider declares /{type}.
	hits := unreachableCalls(c, []scan.FeignClient{client(
		scan.ClientCall{Method: "GET", Path: "/{municipalityId}/ENTERPRISE/partyId"},
	)})
	if len(hits) != 0 {
		t.Errorf("provider path variable should match a hardcoded literal, got %v", paths(hits))
	}
}

func TestUnreachableStripsQueryString(t *testing.T) {
	c := reachCatalog()
	hits := unreachableCalls(c, []scan.FeignClient{client(
		scan.ClientCall{Method: "POST", Path: "/{municipalityId}/messages?async=true"},
	)})
	if len(hits) != 0 {
		t.Errorf("query string should be ignored when matching, got %v", paths(hits))
	}
}

func TestUnreachableSkipsUnresolvedProvider(t *testing.T) {
	c := reachCatalog()
	// Config key resolves to no local service → nothing to compare against.
	hits := unreachableCalls(c, []scan.FeignClient{{
		Package:   "external",
		ConfigKey: "some-saas",
		Calls:     []scan.ClientCall{{Method: "GET", Path: "/whatever"}},
	}})
	if len(hits) != 0 {
		t.Errorf("unresolved provider should be skipped, got %d hits", len(hits))
	}
}
