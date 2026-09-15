// Package catalog holds the in-memory model of the scanned service landscape
// and the query functions shared by the CLI and the MCP server.
package catalog

import "time"

// Catalog is the result of one full scan of the root directory.
type Catalog struct {
	GeneratedAt time.Time `json:"generated_at,omitzero"`
	Root        string    `json:"root"`
	Services    []Service `json:"services"`
	// External lists integration names that resolved to no local repo
	// (third-party systems, services not cloned), sorted and deduplicated.
	External []string `json:"external"`
	// NotRepositories lists directories that look like dept44 services but
	// are not git repositories, so they are excluded from Services. They are
	// reported rather than dropped silently: a stray working folder that
	// shadows a live service is worth seeing.
	NotRepositories []NonRepo `json:"not_repositories,omitempty"`

	byNorm map[string]*Service
}

// NonRepo is a directory whose pom.xml declares the dept44 service parent
// but which is not a git repository, so it is not a service.
type NonRepo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Service is one dept44 microservice repo.
type Service struct {
	Name         string `json:"name"`       // directory name, e.g. api-service-citizen
	ShortName    string `json:"short_name"` // citizen
	Path         string `json:"path"`
	GroupID      string `json:"group_id"`
	ArtifactID   string `json:"artifact_id"`
	Version      string `json:"version"`
	Dept44Parent string `json:"dept44_parent"`
	Description  string `json:"description,omitempty"`
	RepoURL      string `json:"repo_url,omitempty"`
	Org          string `json:"org,omitempty"`

	API      *APIInfo `json:"api,omitempty"`
	SpecPath string   `json:"spec_path,omitempty"` // relative to repo root

	Dependencies []Dependency  `json:"dependencies"`
	Integrations []Integration `json:"integrations"`
	ConsumedBy   []string      `json:"consumed_by"`

	UsesDatabase  bool   `json:"uses_database"`
	DatabaseKind  string `json:"database_kind,omitempty"`
	UsesScheduler bool   `json:"uses_scheduler"`

	Owners    []string `json:"owners,omitempty"`    // from CODEOWNERS
	Workflows []string `json:"workflows,omitempty"` // .github/workflows filenames

	GitHub *RepoStatus `json:"github,omitempty"`

	// Git describes where the local clone sits relative to origin. Machine
	// consumers get this unconditionally — a stale number that looks current
	// is how a fleet sweep gets sent to investigate finished work.
	Git GitState `json:"git_state"`
}

// GitState is the local clone's position relative to origin, read from .git
// without executing git or touching the network.
type GitState struct {
	Branch        string `json:"branch,omitempty"` // empty when detached
	DefaultBranch string `json:"default_branch,omitempty"`
	Detached      bool   `json:"detached,omitempty"`
	HeadSHA       string `json:"head_sha,omitempty"`
	OriginSHA     string `json:"origin_sha,omitempty"`
	// LastFetch is zero when unknown, which never means fresh.
	LastFetch time.Time `json:"last_fetch,omitzero"`

	// WorktreeParent is the dept44 parent version in the checked-out pom,
	// set only when it differs from the authoritative Dept44Parent.
	WorktreeParent string `json:"worktree_parent,omitempty"`
	// OriginVerified is true when Dept44Parent was read from
	// origin/<default> rather than assumed from the worktree.
	OriginVerified bool `json:"origin_verified,omitempty"`
}

// InSyncWithOrigin reports whether the checkout provably matches
// origin/<default>, in which case the worktree's files are origin's files.
func (g GitState) InSyncWithOrigin() bool {
	if g.Detached || g.Branch == "" || g.Branch != g.DefaultBranch {
		return false
	}
	return g.HeadSHA != "" && g.HeadSHA == g.OriginSHA
}

// Drifted reports whether this clone can misrepresent the service.
func (g GitState) Drifted() bool { return !g.InSyncWithOrigin() }

// APIInfo is the subset of the service's own OpenAPI spec we index.
type APIInfo struct {
	Title     string     `json:"title"`
	Version   string     `json:"version"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint is one operation in the service's OpenAPI spec.
type Endpoint struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Dependency is one declared <dependency> in the service pom.
// Version is empty when inherited from the parent.
type Dependency struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version,omitempty"`
	Scope      string `json:"scope,omitempty"`
}

// Integration is an outbound dependency on another service, derived from
// the union of application.yml integration keys, integrations/ spec files,
// and pom openapi-generator inputSpecs.
type Integration struct {
	Name       string   `json:"name"`                  // raw name as found, e.g. "alk-t"
	ResolvedTo string   `json:"resolved_to,omitempty"` // local repo name, "" if external
	Sources    []string `json:"sources"`               // evidence, e.g. "application.yml", "integrations/party-2.0.yml", "pom:inputSpec"
}

// RepoStatus is optional GitHub enrichment (nil unless fetched).
type RepoStatus struct {
	Archived      bool      `json:"archived"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	PushedAt      time.Time `json:"pushed_at,omitzero"`
	LatestRelease string    `json:"latest_release,omitempty"`
}
