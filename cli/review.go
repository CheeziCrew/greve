package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/review"
)

func reviewCmd() *cobra.Command {
	var changed bool
	var base string
	var installHook bool
	var disable []string

	cmd := &cobra.Command{
		Use:   "review [service]",
		Short: "Local dept44 convention linter — exits non-zero on error-severity findings",
		Long: "Runs deterministic dept44 convention checks over a service's working tree " +
			"(or only the files changed vs --base). It is a local pre-commit gate: it touches " +
			"nothing in CI, the Maven build, or GitHub. Error findings make greve exit 1.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}

			var targets []*catalog.Service
			if len(args) == 1 {
				s := c.Lookup(args[0])
				if s == nil {
					return fmt.Errorf("no service matching %q", args[0])
				}
				targets = []*catalog.Service{s}
			} else {
				for i := range c.Services {
					targets = append(targets, &c.Services[i])
				}
			}

			if installHook {
				if len(targets) != 1 {
					return fmt.Errorf("--install-hook needs exactly one service")
				}
				return installPrePushHook(targets[0])
			}

			var results []review.Result
			totalErrors := 0
			for _, s := range targets {
				cfg := review.LoadConfig(s.Path)
				cfg.DisableRules(disable)
				r, err := review.Run(s.Path, s.Name, review.Options{Changed: changed, Base: base}, cfg)
				if err != nil {
					return err
				}
				totalErrors += r.Errors
				// In multi-service mode, only surface services with findings.
				if len(targets) > 1 && len(r.Findings) == 0 {
					continue
				}
				results = append(results, r)
			}

			if jsonOutput {
				if len(targets) == 1 && len(results) == 1 {
					if err := printJSON(results[0]); err != nil {
						return err
					}
				} else if err := printJSON(results); err != nil {
					return err
				}
			} else {
				printReview(results)
			}

			if totalErrors > 0 {
				return fmt.Errorf("review failed: %d error-severity finding(s)", totalErrors)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&changed, "changed", false, "Only lint files changed vs --base (committed since merge-base + working tree)")
	cmd.Flags().StringVar(&base, "base", "main", "Base ref for --changed")
	cmd.Flags().BoolVar(&installHook, "install-hook", false, "Install a local, uncommitted pre-push hook that runs this review")
	cmd.Flags().StringSliceVar(&disable, "disable", nil, "Rule IDs to skip for this run, unioned with .greve-review.yml (repeatable or comma-separated), e.g. --disable no-lombok")
	return cmd
}

// installPrePushHook writes a local-only .git/hooks/pre-push that runs the
// linter on changed files. The hook is never committed (.git/hooks is outside
// the work tree), keeping the gate entirely local.
func installPrePushHook(svc *catalog.Service) error {
	hookDir := filepath.Join(svc.Path, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "pre-push")
	if _, err := os.Stat(hookPath); err == nil {
		return fmt.Errorf("%s already exists — add this line to it yourself:\n  exec greve review %q --changed", hookPath, svc.Name)
	}
	script := "#!/bin/sh\n" +
		"# greve review — local-only dept44 convention gate (uncommitted). Delete this file to disable.\n" +
		"exec greve review \"" + svc.Name + "\" --changed --base \"${GREVE_REVIEW_BASE:-main}\"\n"
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
		return err
	}
	fmt.Printf("installed local pre-push hook: %s\n  runs 'greve review %s --changed' on push; local-only, never committed. Delete the file to disable.\n", hookPath, svc.Name)
	return nil
}

// printReview renders findings grouped by file, with a per-service tally.
func printReview(results []review.Result) {
	for _, r := range results {
		if len(r.Findings) == 0 {
			fmt.Printf("%s — clean (%d files)\n", r.Repo, r.FilesSeen)
			continue
		}
		fmt.Printf("%s — %d error, %d warn, %d info (%d files)\n", r.Repo, r.Errors, r.Warnings, r.Infos, r.FilesSeen)

		byFile := map[string][]review.Finding{}
		var files []string
		for _, f := range r.Findings {
			if _, ok := byFile[f.File]; !ok {
				files = append(files, f.File)
			}
			byFile[f.File] = append(byFile[f.File], f)
		}
		sort.Strings(files)
		for _, file := range files {
			fmt.Printf("\n  %s\n", file)
			for _, f := range byFile[file] {
				fmt.Printf("    %s:%d  [%s] %s  (%s)\n", sevMark(f.Severity), f.Line, f.Severity, f.Message, f.RuleID)
				if f.Snippet != "" {
					fmt.Printf("        > %s\n", f.Snippet)
				}
			}
		}
		fmt.Println()
	}
}

func sevMark(s review.Severity) string {
	switch s {
	case review.SeverityError:
		return "✗"
	case review.SeverityWarn:
		return "!"
	default:
		return "·"
	}
}
