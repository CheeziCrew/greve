// Package insight holds the lazy cross-repo analyses built on top of the
// catalog: change impact, client staleness, integration consistency, usage
// examples, and the ops/test extractors. Everything here reads repo files at
// query time — nothing runs during the base scan.
package insight

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// ImpactHit is one consumer's exposure to a proposed provider change.
type ImpactHit struct {
	Consumer        string `json:"consumer"`
	SpecFile        string `json:"spec_file,omitempty"`
	VendoredVersion string `json:"vendored_version,omitempty"`
	// Affected: "yes" (vendored spec contains the operation/schema),
	// "no" (it does not), or "unknown" (consumer has no vendored spec to check).
	Affected string `json:"affected"`
	Detail   string `json:"detail,omitempty"`
}

// AnalyzeImpact reports, for every consumer of provider, whether its vendored
// client spec contains the given operation (method+path) or component schema.
func AnalyzeImpact(c *catalog.Catalog, provider, method, path, schemaName string) ([]ImpactHit, error) {
	p := c.Lookup(provider)
	if p == nil {
		return nil, fmt.Errorf("no service matching %q", provider)
	}
	if schemaName == "" && (method == "" || path == "") {
		return nil, fmt.Errorf("specify either --schema or both --method and --path")
	}

	var hits []ImpactHit
	for i := range c.Services {
		s := &c.Services[i]
		if s.Name == p.Name {
			continue
		}
		// A consumer may carry several integration entries for the same
		// provider (yml key + spec file under different raw names): gather
		// all of its vendored specs first, and report "unknown" only when
		// the consumer has none at all.
		var specFiles []string
		consumes := false
		for _, integration := range s.Integrations {
			if integration.ResolvedTo != p.Name {
				continue
			}
			consumes = true
			specFiles = append(specFiles, vendoredSpecFiles(integration)...)
		}
		if !consumes {
			continue
		}
		if len(specFiles) == 0 {
			hits = append(hits, ImpactHit{
				Consumer: s.Name,
				Affected: "unknown",
				Detail:   "no vendored client spec to check (config-only integration)",
			})
			continue
		}
		for _, file := range specFiles {
			full := filepath.Join(s.Path, filepath.FromSlash(file))
			hit := ImpactHit{Consumer: s.Name, SpecFile: file}
			if info, err := scan.LoadSpecInfo(full); err == nil {
				hit.VendoredVersion = info.Version
			}
			contains, err := scan.SpecContains(full, method, path, schemaName)
			switch {
			case err != nil:
				hit.Affected = "unknown"
				hit.Detail = err.Error()
			case contains:
				hit.Affected = "yes"
			default:
				hit.Affected = "no"
			}
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

// vendoredSpecFiles extracts the integrations/* sources of an integration.
func vendoredSpecFiles(integration catalog.Integration) []string {
	var files []string
	for _, source := range integration.Sources {
		if strings.HasPrefix(source, "integrations/") {
			files = append(files, "src/main/resources/"+source)
		}
	}
	return files
}
