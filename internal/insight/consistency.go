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
	FeignClients   int      `json:"feign_clients"`
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

	return report
}
