// Package review is greve's local, deterministic dept44 convention linter. It
// walks a service's working tree (or only the files changed vs a base ref),
// runs high-precision lexical rules over each Java source file, and returns
// findings with a non-zero exit when any error-severity rule fires. It is meant
// to run on a developer's machine before commit — it touches nothing in CI, the
// Maven build, or GitHub.
package review

import "sort"

// Severity ranks a finding. Only SeverityError makes greve review exit non-zero.
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Finding is one rule violation at a specific place in a file.
type Finding struct {
	RuleID     string   `json:"rule_id"`
	CorpusID   string   `json:"corpus_id,omitempty"` // cross-ref into the standards corpus
	Severity   Severity `json:"severity"`
	File       string   `json:"file"`           // repo-relative, forward-slash
	Line       int      `json:"line,omitempty"` // 1-based; 0 = whole-file finding
	Message    string   `json:"message"`
	Snippet    string   `json:"snippet,omitempty"`
	SuggestFix string   `json:"suggested_fix,omitempty"`
	Layer      string   `json:"layer,omitempty"`
}

// Result is the outcome of linting one service.
type Result struct {
	Repo      string    `json:"repo"`
	Findings  []Finding `json:"findings"`
	Errors    int       `json:"errors"`
	Warnings  int       `json:"warnings"`
	Infos     int       `json:"infos"`
	FilesSeen int       `json:"files_seen"`
}

// ExitCode is 1 when any error-severity finding exists, else 0.
func (r Result) ExitCode() int {
	if r.Errors > 0 {
		return 1
	}
	return 0
}

// finalize applies config severity overrides, tallies counts, and sorts the
// findings deterministically by file then line then rule.
func (r *Result) finalize(cfg Config) {
	for i := range r.Findings {
		if sev, ok := cfg.Severity[r.Findings[i].RuleID]; ok && sev != "" {
			r.Findings[i].Severity = sev
		}
	}
	r.Errors, r.Warnings, r.Infos = 0, 0, 0
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityError:
			r.Errors++
		case SeverityWarn:
			r.Warnings++
		default:
			r.Infos++
		}
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.RuleID < b.RuleID
	})
}
