package insight

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// ConfigHit is one matching yml entry.
type ConfigHit struct {
	Service string `json:"service"`
	File    string `json:"file"`
	KeyPath string `json:"key_path"`
	Value   string `json:"value"`
}

// SearchConfig greps all application*.yml across the fleet. The query
// matches the dotted key path and, unless keysOnly, the value too.
func SearchConfig(c *catalog.Catalog, query string, keysOnly bool) []ConfigHit {
	q := strings.ToLower(query)
	var hits []ConfigHit

	for i := range c.Services {
		s := &c.Services[i]
		resources := filepath.Join(s.Path, "src", "main", "resources")
		for _, pattern := range []string{"application*.yml", "application*.yaml"} {
			matches, _ := filepath.Glob(filepath.Join(resources, pattern))
			for _, file := range matches {
				doc := loadYaml(file)
				if doc == nil {
					continue
				}
				base := filepath.Base(file)
				walkAllScalars(doc, "", func(keyPath, value string) {
					if strings.Contains(strings.ToLower(keyPath), q) ||
						(!keysOnly && strings.Contains(strings.ToLower(value), q)) {
						hits = append(hits, ConfigHit{Service: s.Name, File: base, KeyPath: keyPath, Value: value})
					}
				})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Service != hits[j].Service {
			return hits[i].Service < hits[j].Service
		}
		return hits[i].KeyPath < hits[j].KeyPath
	})
	return hits
}

// walkAllScalars visits every scalar leaf (string, number, bool) with its
// dotted key path.
func walkAllScalars(node any, prefix string, visit func(keyPath, value string)) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			childPath := k
			if prefix != "" {
				childPath = prefix + "." + k
			}
			walkAllScalars(child, childPath, visit)
		}
	case []any:
		for _, child := range v {
			walkAllScalars(child, prefix, visit)
		}
	case nil:
	default:
		visit(prefix, yamlScalar(v))
	}
}
