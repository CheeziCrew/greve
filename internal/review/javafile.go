package review

import (
	"path"
	"strings"
)

// javaFile is one parsed Java source file. Lines and Sanitized have identical
// length and aligned indices; rules match against Sanitized (comment/string
// content blanked) and report the corresponding Raw line as the snippet.
type javaFile struct {
	Path      string // repo-relative, forward-slash
	Layer     Layer
	Lines     []string // raw lines
	Sanitized []string // comment/string-stripped lines
	sanText   string   // Sanitized joined by "\n", for whole-file checks
}

func newJavaFile(relPath string, raw []byte) *javaFile {
	sanitized := sanitizeLines(raw)
	return &javaFile{
		Path:      relPath,
		Layer:     classify(relPath),
		Lines:     strings.Split(string(raw), "\n"),
		Sanitized: sanitized,
		sanText:   strings.Join(sanitized, "\n"),
	}
}

func (f *javaFile) base() string { return path.Base(f.Path) }

// snippet returns the trimmed raw line (1-based), truncated for display.
func (f *javaFile) snippet(line1 int) string {
	idx := line1 - 1
	if idx < 0 || idx >= len(f.Lines) {
		return ""
	}
	return clip(strings.TrimSpace(f.Lines[idx]), 120)
}

// classDeclLine returns the 1-based line of the first top-level type
// declaration, or 1 if none is found. Used to anchor whole-file findings.
func (f *javaFile) classDeclLine() int {
	for i, s := range f.Sanitized {
		if reTypeDecl.MatchString(s) {
			return i + 1
		}
	}
	return 1
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
