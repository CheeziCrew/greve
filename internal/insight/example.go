package insight

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// maxExcerptLines bounds the size of returned source excerpts.
const maxExcerptLines = 150

// Excerpt is one source file (or fragment) from a real consumer.
type Excerpt struct {
	Role    string `json:"role"` // client | properties | configuration | config-yaml
	Path    string `json:"path"`
	Content string `json:"content"`
}

// UsageExample shows how a real consumer integrates with a provider.
type UsageExample struct {
	Provider string    `json:"provider"`
	Consumer string    `json:"consumer"`
	Excerpts []Excerpt `json:"excerpts"`
}

// FindUsageExample returns the Feign client, its config classes, and the yml
// block from a real consumer of provider. consumer pins the choice; otherwise
// the consumer with the most complete integration (client + vendored spec)
// wins.
func FindUsageExample(c *catalog.Catalog, provider, consumer string) (*UsageExample, error) {
	p := c.Lookup(provider)
	if p == nil {
		return nil, fmt.Errorf("no service matching %q", provider)
	}

	candidates := p.ConsumedBy
	if consumer != "" {
		s := c.Lookup(consumer)
		if s == nil {
			return nil, fmt.Errorf("no service matching %q", consumer)
		}
		candidates = []string{s.Name}
	}

	var best *UsageExample
	bestScore := -1
	for _, name := range candidates {
		s := c.Lookup(name)
		if s == nil {
			continue
		}
		example, score := buildExample(c, p, s)
		if example != nil && score > bestScore {
			best, bestScore = example, score
			if consumer != "" {
				break
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no consumer of %s with a recognizable Feign integration found", p.Name)
	}
	return best, nil
}

func buildExample(c *catalog.Catalog, provider, consumer *catalog.Service) (*UsageExample, int) {
	var match *scan.FeignClient
	for _, client := range scan.FindFeignClients(consumer.Path) {
		key := client.ConfigKey
		if key == "" {
			key = client.Package
		}
		if target := c.ResolveIntegration(key); target != nil && target.Name == provider.Name {
			match = &client
			break
		}
	}
	if match == nil {
		return nil, 0
	}

	example := &UsageExample{Provider: provider.Name, Consumer: consumer.Name}
	score := 1

	addFile := func(role, rel string) {
		if rel == "" {
			return
		}
		content, err := readExcerpt(filepath.Join(consumer.Path, filepath.FromSlash(rel)))
		if err != nil {
			return
		}
		example.Excerpts = append(example.Excerpts, Excerpt{Role: role, Path: rel, Content: content})
		score++
	}
	addFile("client", match.ClientFile)
	addFile("properties", match.PropertiesFile)
	addFile("configuration", match.ConfigurationFile)

	if block := configYamlBlock(consumer.Path, match.ConfigKey); block != "" {
		example.Excerpts = append(example.Excerpts, Excerpt{
			Role:    "config-yaml",
			Path:    "src/main/resources/application.yml",
			Content: block,
		})
		score++
	}
	return example, score
}

func readExcerpt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxExcerptLines {
		lines = append(lines[:maxExcerptLines], fmt.Sprintf("… (%d more lines)", len(lines)-maxExcerptLines))
	}
	return strings.Join(lines, "\n"), nil
}

// configYamlBlock extracts the integration.<key> subtree from the consumer's
// application yml, re-marshalled as standalone yaml.
func configYamlBlock(repoPath, key string) string {
	if key == "" {
		return ""
	}
	for _, name := range []string{"application.yml", "application.yaml"} {
		data, err := os.ReadFile(filepath.Join(repoPath, "src", "main", "resources", name))
		if err != nil {
			continue
		}
		var doc map[string]any
		if yaml.Unmarshal(data, &doc) != nil {
			continue
		}
		integration, ok := doc["integration"].(map[string]any)
		if !ok {
			continue
		}
		block, ok := integration[key]
		if !ok {
			continue
		}
		out, err := yaml.Marshal(map[string]any{"integration": map[string]any{key: block}})
		if err != nil {
			continue
		}
		return string(out)
	}
	return ""
}
