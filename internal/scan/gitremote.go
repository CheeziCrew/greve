package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// gitRemote reads the origin remote URL from .git/config without executing
// git, and returns it normalized to https along with the GitHub org.
func gitRemote(repoPath string) (url, org string) {
	data, err := os.ReadFile(filepath.Join(repoPath, ".git", "config"))
	if err != nil {
		return "", ""
	}

	inOrigin := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = trimmed == `[remote "origin"]`
			continue
		}
		if !inOrigin || !strings.HasPrefix(trimmed, "url") {
			continue
		}
		_, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		return normalizeRemote(strings.TrimSpace(value))
	}
	return "", ""
}

func normalizeRemote(raw string) (url, org string) {
	url = strings.TrimSuffix(raw, ".git")
	if rest, ok := strings.CutPrefix(url, "git@github.com:"); ok {
		url = "https://github.com/" + rest
	}
	if rest, ok := strings.CutPrefix(url, "https://github.com/"); ok {
		org, _, _ = strings.Cut(rest, "/")
	}
	return url, org
}
