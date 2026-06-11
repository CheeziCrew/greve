package insight

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// PatternExample is one real in-org implementation of a component type.
type PatternExample struct {
	Service string `json:"service"`
	File    string `json:"file"` // relative to repo root
	Context string `json:"context,omitempty"`
}

// PatternTypes lists the supported component types.
var PatternTypes = []string{"scheduler", "feign", "validator", "apptest", "mapper", "resource", "entity"}

// maxPatternExamples bounds the result set.
const maxPatternExamples = 3

// FindPatternExamples returns up to three real implementations of a
// component type, preferring services on the newest dept44 parent.
func FindPatternExamples(c *catalog.Catalog, patternType string) []PatternExample {
	var finder func(s *catalog.Service) []PatternExample
	switch patternType {
	case "scheduler":
		finder = findByContent("@Dept44Scheduled", "uses @Dept44Scheduled")
	case "feign":
		finder = func(s *catalog.Service) []PatternExample {
			var out []PatternExample
			for _, client := range scan.FindFeignClients(s.Path) {
				out = append(out, PatternExample{
					Service: s.Name,
					File:    client.ClientFile,
					Context: "Feign client for integration." + client.ConfigKey,
				})
			}
			return out
		}
	case "validator":
		finder = findByPath("api/validation", "Validator.java", "custom constraint validator")
	case "apptest":
		finder = findByContent("@WireMockAppTestSuite", "AppTest extending AbstractAppTest")
	case "mapper":
		finder = findByPath("service/mapper", "Mapper.java", "static mapper class")
	case "resource":
		finder = findByPath("/api", "Resource.java", "REST resource")
	case "entity":
		finder = findByPath("db/model", "Entity.java", "JPA entity")
	default:
		return nil
	}

	// Newest parent first; endpoint count as a maturity tiebreaker.
	ranked := make([]*catalog.Service, 0, len(c.Services))
	for i := range c.Services {
		ranked = append(ranked, &c.Services[i])
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Dept44Parent != ranked[j].Dept44Parent {
			return ranked[i].Dept44Parent > ranked[j].Dept44Parent
		}
		return endpointCount(ranked[i]) > endpointCount(ranked[j])
	})

	var examples []PatternExample
	for _, s := range ranked {
		for _, example := range finder(s) {
			examples = append(examples, example)
			if len(examples) == maxPatternExamples {
				return examples
			}
			break // one example per service keeps the set diverse
		}
	}
	return examples
}

func endpointCount(s *catalog.Service) int {
	if s.API == nil {
		return 0
	}
	return len(s.API.Endpoints)
}

// findByContent returns a finder matching java files containing a marker.
// Test sources are searched for apptest markers, main sources otherwise.
func findByContent(marker, context string) func(*catalog.Service) []PatternExample {
	return func(s *catalog.Service) []PatternExample {
		var out []PatternExample
		roots := []string{filepath.Join(s.Path, "src", "main", "java")}
		if strings.Contains(marker, "AppTest") {
			roots = []string{
				filepath.Join(s.Path, "src", "integration-test", "java"),
				filepath.Join(s.Path, "src", "test", "java"),
			}
		}
		for _, root := range roots {
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".java") || len(out) > 0 {
					return nil
				}
				if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), marker) {
					out = append(out, PatternExample{Service: s.Name, File: relOrBase(s.Path, path), Context: context})
				}
				return nil
			})
		}
		return out
	}
}

// findByPath returns a finder matching files by directory hint + suffix.
func findByPath(dirHint, suffix, context string) func(*catalog.Service) []PatternExample {
	return func(s *catalog.Service) []PatternExample {
		var out []PatternExample
		root := filepath.Join(s.Path, "src", "main", "java")
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || len(out) > 0 {
				return nil
			}
			rel := filepath.ToSlash(path)
			if strings.Contains(rel, dirHint) && strings.HasSuffix(d.Name(), suffix) {
				out = append(out, PatternExample{Service: s.Name, File: relOrBase(s.Path, path), Context: context})
			}
			return nil
		})
		return out
	}
}
