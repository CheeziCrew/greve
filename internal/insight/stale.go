package insight

import (
	"path/filepath"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// StaleHit compares one vendored client spec against the provider's current
// API version.
type StaleHit struct {
	Consumer        string `json:"consumer"`
	Provider        string `json:"provider"`
	SpecFile        string `json:"spec_file"`
	VendoredVersion string `json:"vendored_version"`
	CurrentVersion  string `json:"current_version"`
	Stale           bool   `json:"stale"`
}

// StaleClients lists every vendored client spec whose version differs from
// the provider's current API version. provider filters to one provider when
// non-empty. Providers without a parseable own spec are skipped (no truth to
// compare against).
func StaleClients(c *catalog.Catalog, provider string) []StaleHit {
	var filter *catalog.Service
	if provider != "" {
		filter = c.Lookup(provider)
	}

	var hits []StaleHit
	for i := range c.Services {
		s := &c.Services[i]
		for _, integration := range s.Integrations {
			if integration.ResolvedTo == "" {
				continue
			}
			target := c.Lookup(integration.ResolvedTo)
			if target == nil || target.API == nil || target.API.Version == "" {
				continue
			}
			if filter != nil && target.Name != filter.Name {
				continue
			}
			for _, file := range vendoredSpecFiles(integration) {
				info, err := scan.LoadSpecInfo(filepath.Join(s.Path, filepath.FromSlash(file)))
				if err != nil || info.Version == "" {
					continue
				}
				hits = append(hits, StaleHit{
					Consumer:        s.Name,
					Provider:        target.Name,
					SpecFile:        file,
					VendoredVersion: info.Version,
					CurrentVersion:  target.API.Version,
					Stale:           info.Version != target.API.Version,
				})
			}
		}
	}
	return hits
}
