package review

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/CheeziCrew/greve/internal/insight"
)

// skipDirs are directories never walked: build output, IDE output, agent
// worktrees, VCS, deps, and vendored integration specs. Mirrors the scanner.
var skipDirs = map[string]bool{
	"target":       true,
	"bin":          true,
	".claude":      true,
	".git":         true,
	"node_modules": true,
	"integrations": true,
}

// Options controls one review run.
type Options struct {
	Changed bool   // only lint files changed vs Base
	Base    string // base ref for Changed (default "main")
}

// Run lints one service's main-source tree and returns the findings. Rules run
// over src/main/java only; companion rules consult test sources for context.
func Run(repoPath, repoName string, opts Options, cfg Config) (Result, error) {
	res := Result{Repo: repoName}

	var changed map[string]bool
	if opts.Changed {
		changed = ChangedFiles(repoPath, opts.Base)
	}

	ctx := &scanCtx{TestClasses: testClassNames(repoPath), MigrationColumns: migrationColumns(repoPath, repoName)}

	// Walk the whole repo and key on the path, so multi-module reactors (whose
	// src/main/java lives under each module, not the root) are covered — the
	// same approach as the feign scanner.
	// process lints one file with the rules whose scope matches the source set:
	// main rules over src/main, test rules over src/test & src/integration-test.
	process := func(path string, testSrc bool) {
		rel := relSlash(repoPath, path)
		if changed != nil && !changed[rel] {
			return
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return
		}
		res.FilesSeen++
		jf := newJavaFile(rel, raw)
		for i := range rules {
			r := &rules[i]
			if r.Test != testSrc {
				continue
			}
			if cfg.Disabled[r.ID] {
				continue
			}
			for _, h := range r.check(jf, ctx) {
				res.Findings = append(res.Findings, Finding{
					RuleID:     r.ID,
					CorpusID:   r.CorpusID,
					Severity:   r.Default,
					File:       jf.Path,
					Line:       h.Line,
					Message:    h.Message,
					Snippet:    jf.snippet(h.Line),
					SuggestFix: h.Fix,
					Layer:      string(jf.Layer),
				})
			}
		}
	}

	// Walk the whole repo and key on the path, so multi-module reactors (whose
	// src/main/java lives under each module, not the root) are covered.
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".java") {
			return nil
		}
		slash := filepath.ToSlash(path)
		switch {
		case strings.Contains(slash, "/src/main/java/"):
			process(path, false)
		case strings.Contains(slash, "/src/test/java/"), strings.Contains(slash, "/src/integration-test/java/"):
			process(path, true)
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	res.finalize(cfg)
	return res, nil
}

// testClassNames collects the basenames (without .java) of every test source
// across all modules, used by companion-test rules.
func testClassNames(repoPath string) map[string]bool {
	names := map[string]bool{}
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".java") {
			return nil
		}
		slash := filepath.ToSlash(path)
		if strings.Contains(slash, "/src/test/java/") || strings.Contains(slash, "/src/integration-test/java/") {
			names[strings.TrimSuffix(d.Name(), ".java")] = true
		}
		return nil
	})
	return names
}

// migrationColumns replays the service's Flyway migrations into table -> column -> type
// ("varchar(255)"), so entity rules can cross-check @Column against the real DB column.
// Best-effort: any error yields an empty map and the dependent rule simply no-ops.
func migrationColumns(repoPath, repoName string) map[string]map[string]string {
	out := map[string]map[string]string{}
	schema, err := insight.LoadDBSchema(repoPath, repoName, false)
	if err != nil || schema == nil {
		return out
	}
	for _, t := range schema.Tables {
		cols := map[string]string{}
		for _, c := range t.Columns {
			cols[c.Name] = c.Type
		}
		out[t.Name] = cols
	}
	return out
}

func relSlash(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
