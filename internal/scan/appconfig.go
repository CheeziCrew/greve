package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// appConfig is what the scanner needs from application*.yml/.yaml files.
type appConfig struct {
	integrationKeys map[string][]string // integration name -> files it appeared in
	hasCron         bool
	databaseKind    string // mariadb | postgres | "" — from datasource driver/url
}

// parseAppConfigs reads every src/main/resources/application*.yml|yaml under
// the repo and merges their signals.
func parseAppConfigs(repoPath string) *appConfig {
	cfg := &appConfig{integrationKeys: map[string][]string{}}

	resources := filepath.Join(repoPath, "src", "main", "resources")
	for _, pattern := range []string{"application*.yml", "application*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(resources, pattern))
		for _, file := range matches {
			parseAppConfigFile(file, cfg)
		}
	}
	return cfg
}

func parseAppConfigFile(path string, cfg *appConfig) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return
	}

	base := filepath.Base(path)

	if integration, ok := doc["integration"].(map[string]any); ok {
		for name := range integration {
			cfg.integrationKeys[name] = append(cfg.integrationKeys[name], base)
		}
	}

	if containsKey(doc, "cron") {
		cfg.hasCron = true
	}

	if kind := databaseKind(doc); kind != "" {
		cfg.databaseKind = kind
	}
}

// containsKey reports whether key exists anywhere in the nested yaml document.
func containsKey(node any, key string) bool {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if k == key {
				return true
			}
			if containsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsKey(child, key) {
				return true
			}
		}
	}
	return false
}

func databaseKind(doc map[string]any) string {
	spring, ok := doc["spring"].(map[string]any)
	if !ok {
		return ""
	}
	datasource, ok := spring["datasource"].(map[string]any)
	if !ok {
		return ""
	}
	var hints []string
	if driver, ok := datasource["driver-class-name"].(string); ok {
		hints = append(hints, driver)
	}
	if url, ok := datasource["url"].(string); ok {
		hints = append(hints, url)
	}
	joined := strings.ToLower(strings.Join(hints, " "))
	switch {
	case strings.Contains(joined, "mariadb"):
		return "mariadb"
	case strings.Contains(joined, "postgres"):
		return "postgres"
	case strings.Contains(joined, "mysql"):
		return "mysql"
	case len(hints) > 0:
		return "other"
	}
	return ""
}

// sortedIntegrationNames returns the integration key names in stable order.
func (c *appConfig) sortedIntegrationNames() []string {
	names := make([]string, 0, len(c.integrationKeys))
	for name := range c.integrationKeys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
