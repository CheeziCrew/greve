package catalog

import "strings"

// Filter narrows the service list. Zero-valued fields mean "no constraint".
type Filter struct {
	Query        string // substring match on name, description, api title
	UsesArtifact string // has a declared dependency on this artifactId
	HasDatabase  bool
	HasScheduler bool
	Org          string
}

// FilterServices returns the services matching all set constraints.
func (c *Catalog) FilterServices(f Filter) []*Service {
	var out []*Service
	for i := range c.Services {
		s := &c.Services[i]
		if !matches(s, f) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func matches(s *Service, f Filter) bool {
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		hay := strings.ToLower(s.Name + " " + s.Description + " " + apiTitle(s))
		if !strings.Contains(hay, q) {
			return false
		}
	}
	if f.UsesArtifact != "" {
		found := false
		for _, d := range s.Dependencies {
			if d.ArtifactID == f.UsesArtifact {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.HasDatabase && !s.UsesDatabase {
		return false
	}
	if f.HasScheduler && !s.UsesScheduler {
		return false
	}
	if f.Org != "" && !strings.EqualFold(f.Org, s.Org) {
		return false
	}
	return true
}

func apiTitle(s *Service) string {
	if s.API == nil {
		return ""
	}
	return s.API.Title
}

// Edge is one resolved or unresolved call relation in the service graph.
type Edge struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Resolved bool     `json:"resolved"`
	Sources  []string `json:"sources,omitempty"`
}

// Graph returns the call edges around a service. Direction is "in" (callers),
// "out" (callees), or "both". Depth 1 returns direct neighbours; higher
// depths follow resolved edges transitively. If name is empty, all edges in
// the catalog are returned and direction/depth are ignored.
func (c *Catalog) Graph(name, direction string, depth int) []Edge {
	if name == "" {
		return c.allEdges()
	}
	start := c.Lookup(name)
	if start == nil {
		return nil
	}
	if depth < 1 {
		depth = 1
	}

	var edges []Edge
	seen := map[string]bool{}
	frontier := []string{start.Name}

	for range depth {
		var next []string
		for _, current := range frontier {
			if seen[current] {
				continue
			}
			seen[current] = true
			s := c.Lookup(current)
			if s == nil {
				continue
			}
			if direction == "out" || direction == "both" {
				for _, integration := range s.Integrations {
					to := integration.ResolvedTo
					resolved := to != ""
					if !resolved {
						to = integration.Name
					}
					edges = append(edges, Edge{From: s.Name, To: to, Resolved: resolved, Sources: integration.Sources})
					if resolved && !seen[to] {
						next = append(next, to)
					}
				}
			}
			if direction == "in" || direction == "both" {
				for _, caller := range s.ConsumedBy {
					edges = append(edges, Edge{From: caller, To: s.Name, Resolved: true})
					if !seen[caller] {
						next = append(next, caller)
					}
				}
			}
		}
		frontier = next
	}
	return dedupeEdges(edges)
}

func (c *Catalog) allEdges() []Edge {
	var edges []Edge
	for i := range c.Services {
		s := &c.Services[i]
		for _, integration := range s.Integrations {
			to := integration.ResolvedTo
			resolved := to != ""
			if !resolved {
				to = integration.Name
			}
			edges = append(edges, Edge{From: s.Name, To: to, Resolved: resolved, Sources: integration.Sources})
		}
	}
	return edges
}

func dedupeEdges(edges []Edge) []Edge {
	seen := map[string]bool{}
	var out []Edge
	for _, e := range edges {
		key := e.From + "->" + e.To
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

// EndpointHit is one endpoint search result.
type EndpointHit struct {
	Service     string   `json:"service"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// SearchEndpoints finds endpoints whose path, operationId, or summary
// contains the query (case-insensitive). Method, when set, must match.
func (c *Catalog) SearchEndpoints(query, method string) []EndpointHit {
	q := strings.ToLower(query)
	method = strings.ToUpper(method)
	var hits []EndpointHit
	for i := range c.Services {
		s := &c.Services[i]
		if s.API == nil {
			continue
		}
		for _, ep := range s.API.Endpoints {
			if method != "" && ep.Method != method {
				continue
			}
			hay := strings.ToLower(ep.Path + " " + ep.OperationID + " " + ep.Summary)
			if !strings.Contains(hay, q) {
				continue
			}
			hits = append(hits, EndpointHit{
				Service:     s.Name,
				Method:      ep.Method,
				Path:        ep.Path,
				OperationID: ep.OperationID,
				Summary:     ep.Summary,
				Tags:        ep.Tags,
			})
		}
	}
	return hits
}

// VersionHit is one dependency-version search result.
type VersionHit struct {
	Service   string `json:"service"`
	GroupID   string `json:"group_id"`
	Artifact  string `json:"artifact"`
	Version   string `json:"version"`
	Inherited bool   `json:"inherited"`
}

// DependencyVersions reports which services depend on the given artifactId
// and at what version. The dept44 parent is special-cased since it is not a
// regular dependency. versionPrefix, when set, filters results.
func (c *Catalog) DependencyVersions(artifact, versionPrefix string) []VersionHit {
	var hits []VersionHit
	for i := range c.Services {
		s := &c.Services[i]
		if artifact == "dept44-service-parent" {
			if strings.HasPrefix(s.Dept44Parent, versionPrefix) {
				hits = append(hits, VersionHit{
					Service:  s.Name,
					GroupID:  "se.sundsvall.dept44",
					Artifact: artifact,
					Version:  s.Dept44Parent,
				})
			}
			continue
		}
		for _, d := range s.Dependencies {
			if d.ArtifactID != artifact {
				continue
			}
			version := d.Version
			inherited := version == ""
			if inherited {
				version = "(inherited)"
			}
			if versionPrefix != "" && !strings.HasPrefix(version, versionPrefix) {
				continue
			}
			hits = append(hits, VersionHit{
				Service:   s.Name,
				GroupID:   d.GroupID,
				Artifact:  d.ArtifactID,
				Version:   version,
				Inherited: inherited,
			})
		}
	}
	return hits
}
