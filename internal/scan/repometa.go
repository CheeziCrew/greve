package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codeOwners parses .github/CODEOWNERS and returns the unique owners.
func codeOwners(repoPath string) []string {
	data, err := os.ReadFile(filepath.Join(repoPath, ".github", "CODEOWNERS"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join(repoPath, "CODEOWNERS"))
		if err != nil {
			return nil
		}
	}

	seen := map[string]bool{}
	var owners []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, field := range strings.Fields(line)[1:] {
			if strings.HasPrefix(field, "@") && !seen[field] {
				seen[field] = true
				owners = append(owners, field)
			}
		}
	}
	sort.Strings(owners)
	return owners
}

// workflowNames lists the .github/workflows files of a repo.
func workflowNames(repoPath string) []string {
	entries, err := os.ReadDir(filepath.Join(repoPath, ".github", "workflows"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
