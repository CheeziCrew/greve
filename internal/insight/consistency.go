package insight

import (
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// ConsistencyReport lists integration drift within one service.
type ConsistencyReport struct {
	Service string `json:"service"`
	// Feign clients whose config key matches no integration.* yml key.
	ClientWithoutConfig []string `json:"client_without_config,omitempty"`
	// integration.* yml keys with no matching Feign client. Services using
	// WebClient instead of Feign land here — heuristic, not a verdict.
	ConfigWithoutClient []string `json:"config_without_client,omitempty"`
	// Vendored specs in integrations/ not referenced by any pom inputSpec
	// (no client model generated from them).
	SpecWithoutPom []string `json:"spec_without_pom,omitempty"`
	// Feign client routes that match no endpoint the resolved provider serves
	// in its own OpenAPI spec — a broken call (renamed/removed provider path).
	// Heuristic: only checked when the provider resolves locally and has a
	// parseable spec; a stale provider spec can also surface here.
	UnreachableCalls []UnreachableCall `json:"unreachable_calls,omitempty"`
	FeignClients     int               `json:"feign_clients"`
}

// UnreachableCall is one Feign route whose path/verb the provider does not
// serve in its current spec.
type UnreachableCall struct {
	Client   string `json:"client"`   // consumer's client package
	Provider string `json:"provider"` // resolved provider service
	Method   string `json:"method,omitempty"`
	Path     string `json:"path"`
}

// HasDrift reports whether the service has any consistency finding worth
// surfacing.
func (r ConsistencyReport) HasDrift() bool {
	return len(r.ClientWithoutConfig)+len(r.ConfigWithoutClient)+
		len(r.SpecWithoutPom)+len(r.UnreachableCalls) > 0
}

// CheckConsistency cross-references the integration signals of one service.
func CheckConsistency(c *catalog.Catalog, s *catalog.Service) ConsistencyReport {
	report := ConsistencyReport{Service: s.Name}

	clients := scan.FindFeignClients(s.Path)
	report.FeignClients = len(clients)

	ymlKeys := map[string]bool{}
	for _, integration := range s.Integrations {
		for _, source := range integration.Sources {
			if strings.HasSuffix(source, ".yml") || strings.HasSuffix(source, ".yaml") {
				if !strings.HasPrefix(source, "integrations/") {
					ymlKeys[catalog.Normalize(integration.Name)] = true
				}
			}
		}
	}

	clientKeys := map[string]bool{}
	for _, client := range clients {
		key := catalog.Normalize(client.ConfigKey)
		if key == "" {
			key = catalog.Normalize(client.Package)
		}
		clientKeys[key] = true
		if !ymlKeys[key] {
			report.ClientWithoutConfig = append(report.ClientWithoutConfig, client.Package)
		}
	}
	for _, integration := range s.Integrations {
		key := catalog.Normalize(integration.Name)
		if ymlKeys[key] && !clientKeys[key] {
			report.ConfigWithoutClient = append(report.ConfigWithoutClient, integration.Name)
		}
	}

	for _, integration := range s.Integrations {
		hasSpec, hasPom := false, false
		for _, source := range integration.Sources {
			if strings.HasPrefix(source, "integrations/") {
				hasSpec = true
			}
			if source == "pom:inputSpec" {
				hasPom = true
			}
		}
		if hasSpec && !hasPom {
			report.SpecWithoutPom = append(report.SpecWithoutPom, integration.Name)
		}
	}

	report.UnreachableCalls = unreachableCalls(c, clients)

	return report
}

// unreachableCalls flags Feign routes whose verb+path the resolved provider
// does not serve in its own OpenAPI spec — the broken-route check that catches
// a provider renaming/removing a path under a consumer that still calls it.
//
// Skipped when the provider can't be resolved locally or has no parseable spec
// (no ground truth). To avoid the context-path false positive (a consumer URL
// like .../api/v3 makes every raw client path mismatch the provider's spec
// paths), a client's unmatched routes are only reported when at least one of
// its routes DOES match that provider — i.e. the base paths align, so a single
// stray route is real drift rather than a whole-client offset.
func unreachableCalls(c *catalog.Catalog, clients []scan.FeignClient) []UnreachableCall {
	var hits []UnreachableCall
	for _, client := range clients {
		if client.ConfigKey == "" || len(client.Calls) == 0 {
			continue
		}
		provider := c.ResolveIntegration(client.ConfigKey)
		if provider == nil || provider.API == nil || len(provider.API.Endpoints) == 0 {
			continue
		}

		var unmatched []UnreachableCall
		matched := 0
		for _, call := range client.Calls {
			if servedBy(provider.API.Endpoints, call) {
				matched++
				continue
			}
			unmatched = append(unmatched, UnreachableCall{
				Client:   client.Package,
				Provider: provider.Name,
				Method:   call.Method,
				Path:     call.Path,
			})
		}
		if matched > 0 {
			hits = append(hits, unmatched...)
		}
	}
	return hits
}

// servedBy reports whether the provider serves a route matching this call:
// same verb (or the call declares none) and a path the provider exposes,
// matched segment-by-segment.
func servedBy(endpoints []catalog.Endpoint, call scan.ClientCall) bool {
	callSegs := pathSegments(call.Path)
	for _, ep := range endpoints {
		if call.Method != "" && ep.Method != call.Method {
			continue
		}
		if segmentsMatch(pathSegments(ep.Path), callSegs) {
			return true
		}
	}
	return false
}

// segmentsMatch compares two paths position by position. A path variable on
// either side ("{}") is a wildcard, so {id} vs {errandId} match and a literal
// the consumer hardcodes where the provider declares a variable (party's
// /{type} called as /ENTERPRISE) is not a false drift.
func segmentsMatch(provider, client []string) bool {
	if len(provider) != len(client) {
		return false
	}
	for i := range provider {
		if provider[i] == "{}" || client[i] == "{}" {
			continue
		}
		if provider[i] != client[i] {
			return false
		}
	}
	return true
}

// pathSegments strips any query string, splits on "/", and collapses every
// path-variable segment to "{}".
func pathSegments(p string) []string {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			segments[i] = "{}"
		}
	}
	return segments
}
