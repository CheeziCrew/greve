package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// specCandidates lists known OpenAPI spec locations in preference order:
// main resources beat test resources beat integration-test resources.
var specCandidates = []string{
	"src/main/resources/api/openapi.yaml",
	"src/main/resources/api/openapi.yml",
	"src/main/resources/openapi.yaml",
	"src/main/resources/openapi.yml",
	"src/main/resources/META-INF/openapi.yaml",
	"src/main/resources/META-INF/openapi.yml",
	"src/test/resources/api/openapi.yaml",
	"src/test/resources/api/openapi.yml",
	"src/test/resources/openapi.yaml",
	"src/test/resources/openapi.yml",
	"src/integration-test/resources/api/openapi.yaml",
	"src/integration-test/resources/api/openapi.yml",
	"src/integration-test/resources/openapi.yaml",
	"src/integration-test/resources/openapi.yml",
}

// skipDirs are directories never searched in the fallback walk: build output,
// IDE output, agent worktrees, and consumed-service specs.
var skipDirs = map[string]bool{
	"target":       true,
	"bin":          true,
	".claude":      true,
	".git":         true,
	"node_modules": true,
	"integrations": true,
}

// findSpec locates the service's own OpenAPI spec. Returns the path relative
// to the repo root, or "".
func findSpec(repoPath string) string {
	for _, candidate := range specCandidates {
		full := filepath.Join(repoPath, filepath.FromSlash(candidate))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return candidate
		}
	}

	// Bounded fallback: walk src/ for openapi.y(a)ml in unexpected spots.
	var found []string
	src := filepath.Join(repoPath, "src")
	_ = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if name == "openapi.yaml" || name == "openapi.yml" {
			if rel, err := filepath.Rel(repoPath, path); err == nil {
				found = append(found, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if len(found) == 0 {
		return ""
	}
	sort.Strings(found) // deterministic; "main" sorts before "test" within src/
	return found[0]
}

// httpMethods are the OpenAPI path-item keys that denote operations.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// parseSpec extracts info + paths from an OpenAPI yaml without a full
// OpenAPI library.
func parseSpec(path string) (*catalog.APIInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Info struct {
			Title   string `yaml:"title"`
			Version string `yaml:"version"`
		} `yaml:"info"`
		Paths map[string]map[string]struct {
			OperationID string   `yaml:"operationId"`
			Summary     string   `yaml:"summary"`
			Tags        []string `yaml:"tags"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	api := &catalog.APIInfo{Title: doc.Info.Title, Version: doc.Info.Version}
	for p, item := range doc.Paths {
		for _, method := range httpMethods {
			op, ok := item[method]
			if !ok {
				continue
			}
			api.Endpoints = append(api.Endpoints, catalog.Endpoint{
				Method:      strings.ToUpper(method),
				Path:        p,
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Tags:        op.Tags,
			})
		}
	}
	sort.Slice(api.Endpoints, func(i, j int) bool {
		if api.Endpoints[i].Path != api.Endpoints[j].Path {
			return api.Endpoints[i].Path < api.Endpoints[j].Path
		}
		return api.Endpoints[i].Method < api.Endpoints[j].Method
	})
	return api, nil
}

// listIntegrationSpecs returns the basenames of specs under
// src/main/resources/integrations/.
func listIntegrationSpecs(repoPath string) []string {
	dir := filepath.Join(repoPath, "src", "main", "resources", "integrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
