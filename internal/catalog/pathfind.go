package catalog

// maxPathDepth bounds the BFS for PathBetween.
const maxPathDepth = 5

// PathBetween returns the shortest call chain from one service to another
// over resolved edges, or nil when none exists within maxPathDepth hops.
func (c *Catalog) PathBetween(from, to string) []string {
	start, goal := c.Lookup(from), c.Lookup(to)
	if start == nil || goal == nil {
		return nil
	}
	if start.Name == goal.Name {
		return []string{start.Name}
	}

	adjacency := map[string][]string{}
	for _, edge := range c.allEdges() {
		if edge.Resolved {
			adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		}
	}

	parent := map[string]string{start.Name: ""}
	frontier := []string{start.Name}
	for depth := 0; depth < maxPathDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, current := range frontier {
			for _, neighbor := range adjacency[current] {
				if _, visited := parent[neighbor]; visited {
					continue
				}
				parent[neighbor] = current
				if neighbor == goal.Name {
					return tracePath(parent, neighbor)
				}
				next = append(next, neighbor)
			}
		}
		frontier = next
	}
	return nil
}

func tracePath(parent map[string]string, end string) []string {
	var path []string
	for current := end; current != ""; current = parent[current] {
		path = append([]string{current}, path...)
	}
	return path
}
