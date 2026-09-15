// Package testfixtures materializes the testdata repo fixtures for tests.
//
// Git refuses to track a nested literal .git directory, so the fixtures under
// testdata/repos carry theirs as dotgit/. Scan needs a real .git to tell a
// service from a stray working copy, so tests copy the tree and restore the
// name rather than scanning testdata in place.
package testfixtures

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Materialize copies src into dst, renaming every dotgit path segment to .git.
func Materialize(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		for i, part := range parts {
			if part == "dotgit" {
				parts[i] = ".git"
			}
		}
		target := filepath.Join(dst, filepath.Join(parts...))

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
