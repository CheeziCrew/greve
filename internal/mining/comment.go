// Package mining pulls human PR review comments across the dept44 Java repos via
// the GitHub GraphQL API (through the authenticated gh CLI) and stores the
// substantive ones locally. greve's job ends at mine + store; the distillation
// of these comments into the standards corpus is done by a separate Claude
// Workflow that reads the stored file.
package mining

// Comment is one stored PR review comment.
type Comment struct {
	ID          string `json:"id"` // GraphQL node id — stable dedup key
	Org         string `json:"org"`
	Repo        string `json:"repo"`
	PR          int    `json:"pr"`
	Author      string `json:"author"`
	Association string `json:"association"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Body        string `json:"body"`
	DiffHunk    string `json:"diff_hunk,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// Repo is one repository to mine.
type Repo struct {
	Org  string
	Name string
}

func (r Repo) Key() string { return r.Org + "/" + r.Name }
