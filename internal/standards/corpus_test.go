package standards

import (
	"testing"

	"github.com/CheeziCrew/greve/internal/review"
)

func TestEmbeddedCorpusLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Rules) == 0 {
		t.Fatal("embedded corpus is empty")
	}
	seen := map[string]bool{}
	for _, r := range c.Rules {
		if r.ID == "" || r.Category == "" || r.Statement == "" {
			t.Errorf("incomplete rule: %+v", r)
		}
		if seen[r.ID] {
			t.Errorf("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
	}
}

// TestEveryLinterRuleHasCorpusEntry keeps the deterministic linter (Capability A)
// and the rulebook (Capability B) from drifting apart.
func TestEveryLinterRuleHasCorpusEntry(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ids := map[string]bool{}
	for _, r := range c.Rules {
		ids[r.ID] = true
	}
	for ruleID, corpusID := range review.RuleRefs() {
		if corpusID == "" {
			t.Errorf("linter rule %q has no corpus id", ruleID)
			continue
		}
		if !ids[corpusID] {
			t.Errorf("linter rule %q references missing corpus id %q", ruleID, corpusID)
		}
	}
}

func TestForPathInfersLayer(t *testing.T) {
	cases := map[string]string{
		"src/main/java/se/x/api/DemoResource.java":                "resource",
		"src/main/java/se/x/api/model/Demo.java":                  "pojo",
		"src/main/java/se/x/integration/db/model/DemoEntity.java": "entity",
		"src/main/java/se/x/service/mapper/DemoMapper.java":       "mapper",
	}
	c, _ := Load()
	for path, wantLayer := range cases {
		layer, rules := c.ForPath(path)
		if layer != wantLayer {
			t.Errorf("ForPath(%s) layer=%s want %s", path, layer, wantLayer)
		}
		if len(rules) == 0 {
			t.Errorf("ForPath(%s) returned no rules", path)
		}
	}
}
