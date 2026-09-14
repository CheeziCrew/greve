package review

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the optional per-repo .greve-review.yml. It is repo-local and
// uncommitted by intent — the linter is a local gate, not a CI check.
type Config struct {
	Disabled map[string]bool
	Severity map[string]Severity
}

// LoadConfig reads <repo>/.greve-review.yml if present. Missing file = all rules
// enabled at their default severity.
func LoadConfig(repoPath string) Config {
	cfg := Config{Disabled: map[string]bool{}, Severity: map[string]Severity{}}
	data, err := os.ReadFile(filepath.Join(repoPath, ".greve-review.yml"))
	if err != nil {
		return cfg
	}
	var raw struct {
		Disable  []string          `yaml:"disable"`
		Severity map[string]string `yaml:"severity"`
	}
	if yaml.Unmarshal(data, &raw) != nil {
		return cfg
	}
	for _, id := range raw.Disable {
		cfg.Disabled[id] = true
	}
	for id, sev := range raw.Severity {
		cfg.Severity[id] = Severity(sev)
	}
	return cfg
}

// DisableRules marks additional rule IDs as disabled for this run, unioned on
// top of whatever the repo's .greve-review.yml already disabled. Backs the
// CLI/MCP --disable flag for ad-hoc suppression — e.g. a service where Lombok
// is an accepted local exception (--disable no-lombok).
func (c *Config) DisableRules(ids []string) {
	if c.Disabled == nil {
		c.Disabled = map[string]bool{}
	}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			c.Disabled[id] = true
		}
	}
}
