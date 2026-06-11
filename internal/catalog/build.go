package catalog

import (
	"slices"
	"sort"
	"strings"
	"time"
)

// Normalize reduces a service or integration name to its comparable core:
// lowercase, alphanumerics only. This makes "alk-t" match "alkt" and
// "case-data" match "casedata".
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Build assembles a Catalog from scanned services: resolves integration
// names against the service set, derives reverse (ConsumedBy) edges, and
// collects unresolved names as external systems. The aliases map (raw
// integration name -> repo name) is an escape hatch for names that defeat
// normalization; it may be nil.
func Build(root string, services []Service, aliases map[string]string) *Catalog {
	c := &Catalog{
		GeneratedAt: time.Now().UTC(),
		Root:        root,
		Services:    services,
		byNorm:      map[string]*Service{},
	}

	for i := range c.Services {
		s := &c.Services[i]
		for _, key := range []string{s.Name, s.ShortName, s.ArtifactID} {
			if n := Normalize(key); n != "" {
				c.byNorm[n] = s
			}
		}
	}
	// Secondary keys with common repo-name affixes stripped, so e.g.
	// integration "comfactfacade" finds repo "api-comfact-facade". These
	// never clobber a primary key.
	for i := range c.Services {
		s := &c.Services[i]
		for _, variant := range affixVariants(s.Name) {
			if n := Normalize(variant); n != "" {
				if _, taken := c.byNorm[n]; !taken {
					c.byNorm[n] = s
				}
			}
		}
	}

	consumedBy := map[string][]string{}
	external := map[string]bool{}

	for i := range c.Services {
		s := &c.Services[i]
		for j := range s.Integrations {
			integration := &s.Integrations[j]
			target := c.resolve(integration.Name, aliases)
			if target == nil {
				external[integration.Name] = true
				continue
			}
			integration.ResolvedTo = target.Name
			if target.Name != s.Name {
				consumedBy[target.Name] = append(consumedBy[target.Name], s.Name)
			}
		}
	}

	for i := range c.Services {
		s := &c.Services[i]
		callers := consumedBy[s.Name]
		sort.Strings(callers)
		s.ConsumedBy = slices.Compact(callers)
		// Empty slices, not nulls, in the exported JSON.
		if s.ConsumedBy == nil {
			s.ConsumedBy = []string{}
		}
		if s.Dependencies == nil {
			s.Dependencies = []Dependency{}
		}
		if s.Integrations == nil {
			s.Integrations = []Integration{}
		}
	}
	if c.External == nil {
		c.External = []string{}
	}

	for name := range external {
		c.External = append(c.External, name)
	}
	sort.Strings(c.External)

	return c
}

// affixVariants returns the repo name with common prefixes/suffixes removed.
func affixVariants(name string) []string {
	var variants []string
	for _, prefix := range []string{"api-service-", "api-", "pw-"} {
		if stripped, ok := strings.CutPrefix(name, prefix); ok {
			variants = append(variants, stripped)
			if tail, ok := strings.CutSuffix(stripped, "-service"); ok {
				variants = append(variants, tail)
			}
		}
	}
	if stripped, ok := strings.CutSuffix(name, "-service"); ok {
		variants = append(variants, stripped)
	}
	return variants
}

// resolve maps an integration name to a local service, trying the alias map,
// the normalized name, and api-affix-stripped variants ("messaging-api",
// "api-messaging" -> "messaging").
func (c *Catalog) resolve(name string, aliases map[string]string) *Service {
	if alias, ok := aliases[name]; ok {
		if s, ok := c.byNorm[Normalize(alias)]; ok {
			return s
		}
	}
	candidates := []string{
		Normalize(name),
		Normalize(strings.TrimPrefix(name, "api-")),
		Normalize(strings.TrimSuffix(name, "-api")),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if s, ok := c.byNorm[candidate]; ok {
			return s
		}
	}
	return nil
}

// Lookup finds a service by fuzzy name: exact directory name, short name,
// or artifactId, all normalized.
func (c *Catalog) Lookup(name string) *Service {
	if s, ok := c.byNorm[Normalize(name)]; ok {
		return s
	}
	return nil
}

// ResolveIntegration maps an integration name (e.g. a Feign config key) to a
// local service using the same rules as the build-time edge resolution.
func (c *Catalog) ResolveIntegration(name string) *Service {
	return c.resolve(name, nil)
}
