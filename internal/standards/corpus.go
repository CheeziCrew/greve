// Package standards serves the dept44 convention rulebook: a machine-consumable
// distillation of the prose conventions (CLAUDE.md + the pattern-*.md files) and
// — once PR mining runs — the recurring human review feedback. A baseline corpus
// is embedded in the binary; an override at <UserConfigDir>/greve/standards-corpus.json
// (written by the mining distillation) wins by rule id.
package standards

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/CheeziCrew/greve/internal/review"
)

//go:embed standards-corpus.json
var baseline []byte

// Rule is one convention entry.
type Rule struct {
	ID            string `json:"id"`
	Category      string `json:"category"` // layer: resource|pojo|entity|repository|mapper|integration|scheduler|service|validation|apptest|general
	Statement     string `json:"statement"`
	Rationale     string `json:"rationale,omitempty"`
	BadExample    string `json:"bad_example,omitempty"`
	GoodExample   string `json:"good_example,omitempty"`
	Frequency     int    `json:"frequency,omitempty"` // times seen in mined PR comments
	Source        string `json:"source"`              // "CLAUDE.md" | "pattern-*.md" | "pr-mining"
	Deterministic bool   `json:"deterministic"`       // enforced by a greve review rule
	LinterRuleID  string `json:"linter_rule_id,omitempty"`
}

// Corpus is the full rulebook.
type Corpus struct {
	Rules []Rule `json:"rules"`
}

// Load returns the embedded baseline merged with the user override (override
// rules replace baseline rules of the same id; new ids are appended).
func Load() (*Corpus, error) {
	var c Corpus
	if err := json.Unmarshal(baseline, &c); err != nil {
		return nil, err
	}
	if dir, err := os.UserConfigDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, "greve", "standards-corpus.json")); err == nil {
			var override Corpus
			if json.Unmarshal(data, &override) == nil {
				c.merge(override)
			}
		}
	}
	sort.SliceStable(c.Rules, func(i, j int) bool {
		if c.Rules[i].Category != c.Rules[j].Category {
			return c.Rules[i].Category < c.Rules[j].Category
		}
		return c.Rules[i].ID < c.Rules[j].ID
	})
	return &c, nil
}

func (c *Corpus) merge(override Corpus) {
	idx := map[string]int{}
	for i, r := range c.Rules {
		idx[r.ID] = i
	}
	for _, r := range override.Rules {
		if i, ok := idx[r.ID]; ok {
			c.Rules[i] = r
		} else {
			idx[r.ID] = len(c.Rules)
			c.Rules = append(c.Rules, r)
		}
	}
}

// ByCategory returns the rules in a category, or all rules when category is "".
func (c *Corpus) ByCategory(category string) []Rule {
	if category == "" {
		return c.Rules
	}
	var out []Rule
	for _, r := range c.Rules {
		if r.Category == category {
			out = append(out, r)
		}
	}
	return out
}

// ForPath infers a file's layer and returns the rules that apply to it (its
// layer plus the always-applicable "general" rules).
func (c *Corpus) ForPath(path string) (category string, rules []Rule) {
	category = review.ClassifyPath(path)
	for _, r := range c.Rules {
		if r.Category == category || r.Category == "general" {
			rules = append(rules, r)
		}
	}
	return category, rules
}
