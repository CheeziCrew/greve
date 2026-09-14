package review

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDisableRuleSuppressesFindings covers the --disable / .greve-review.yml
// suppression path end-to-end through Run: a rule listed in cfg.Disabled must
// not produce findings, while the same tree lints normally without it.
func TestDisableRuleSuppressesFindings(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src", "main", "java", "se", "x", "model")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	entity := "package se.x.model;\n\nimport lombok.Data;\n\n@Data\npublic class Foo {}\n"
	if err := os.WriteFile(filepath.Join(src, "Foo.java"), []byte(entity), 0o644); err != nil {
		t.Fatal(err)
	}

	count := func(cfg Config) int {
		res, err := Run(tmp, "svc", Options{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range res.Findings {
			if f.RuleID == "no-lombok" {
				n++
			}
		}
		return n
	}

	// Baseline: the Lombok import fires no-lombok.
	if got := count(LoadConfig(tmp)); got == 0 {
		t.Fatal("expected no-lombok to fire without suppression")
	}

	// Disabled via the flag helper: no findings.
	cfg := LoadConfig(tmp)
	cfg.DisableRules([]string{" no-lombok "}) // also asserts trimming
	if got := count(cfg); got != 0 {
		t.Fatalf("expected no-lombok suppressed, got %d finding(s)", got)
	}
}

// TestDisableRulesInitializesNilMap guards the pointer-receiver lazy-init so a
// zero-value Config (not built via LoadConfig) is safe to disable rules on.
func TestDisableRulesInitializesNilMap(t *testing.T) {
	var cfg Config
	cfg.DisableRules([]string{"no-lombok"})
	if !cfg.Disabled["no-lombok"] {
		t.Fatal("DisableRules should lazily initialize Disabled and set the rule")
	}
}
