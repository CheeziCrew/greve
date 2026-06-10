package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// readmeDescription returns the first real paragraph of the repo README:
// the first line that is not a heading, badge, image, HTML tag, or blank.
func readmeDescription(repoPath string) string {
	for _, name := range []string{"README.md", "readme.md", "README.MD"} {
		data, err := os.ReadFile(filepath.Join(repoPath, name))
		if err != nil {
			continue
		}
		if desc := firstParagraph(string(data)); desc != "" {
			return desc
		}
	}
	return ""
}

func firstParagraph(content string) string {
	var paragraph []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if isNoise(line) {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		paragraph = append(paragraph, line)
	}
	// Strip markdown emphasis wrapping the whole paragraph (_..._ or *...*).
	return strings.Trim(strings.Join(paragraph, " "), "*_ ")
}

func isNoise(line string) bool {
	if line == "" || line == "---" {
		return true
	}
	for _, prefix := range []string{"#", "![", "[!", "<", "|", "```", "> "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}
