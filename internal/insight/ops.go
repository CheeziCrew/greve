package insight

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// EnvVar is one ${VAR} or ${VAR:default} placeholder in a service's base
// application yml.
type EnvVar struct {
	Name    string `json:"name"`
	Default string `json:"default,omitempty"`
	HasDef  bool   `json:"has_default"`
	KeyPath string `json:"key_path"` // dotted yml path where it is used
}

// ConfigSurface is the deploy checklist of one service.
type ConfigSurface struct {
	Service string   `json:"service"`
	EnvVars []EnvVar `json:"env_vars"`
}

var placeholderRe = regexp.MustCompile(`\$\{([A-Za-z0-9_.-]+?)(?::([^}]*))?\}`)

// LoadConfigSurface extracts placeholders from the base application yml
// (deploy profile — the -it/-junit test profiles are excluded on purpose).
func LoadConfigSurface(s *catalog.Service) *ConfigSurface {
	surface := &ConfigSurface{Service: s.Name, EnvVars: []EnvVar{}}
	for _, name := range []string{"application.yml", "application.yaml"} {
		doc := loadYaml(filepath.Join(s.Path, "src", "main", "resources", name))
		if doc == nil {
			continue
		}
		walkStrings(doc, "", func(keyPath, value string) {
			for _, m := range placeholderRe.FindAllStringSubmatch(value, -1) {
				// Spring-style ${spring.application.name} self-references are
				// config lookups, not env vars: keep only env-var-shaped names.
				if strings.ToUpper(m[1]) != m[1] {
					continue
				}
				surface.EnvVars = append(surface.EnvVars, EnvVar{
					Name:    m[1],
					Default: m[2],
					HasDef:  strings.Contains(m[0], ":"),
					KeyPath: keyPath,
				})
			}
		})
	}
	sort.Slice(surface.EnvVars, func(i, j int) bool { return surface.EnvVars[i].Name < surface.EnvVars[j].Name })
	return surface
}

// Job is one scheduled job.
type Job struct {
	Service string `json:"service"`
	Name    string `json:"name"`   // config path or annotation name
	Cron    string `json:"cron"`   // expression, resolved from config when referenced
	Source  string `json:"source"` // "application.yml" or the java file
}

var dept44ScheduledRe = regexp.MustCompile(`(?s)@Dept44Scheduled\s*\((.*?)\)\s*\n`)
var annotationAttrRe = regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)

// LoadJobs collects cron jobs of one service from yml cron keys and
// @Dept44Scheduled annotations (lazy Java scan).
func LoadJobs(s *catalog.Service) []Job {
	var jobs []Job
	seen := map[string]bool{}
	values := map[string]string{}

	for _, name := range []string{"application.yml", "application.yaml"} {
		file := filepath.Join(s.Path, "src", "main", "resources", name)
		doc := loadYaml(file)
		if doc == nil {
			continue
		}
		walkStrings(doc, "", func(keyPath, value string) {
			values[keyPath] = value
			if strings.HasSuffix(keyPath, ".cron") || keyPath == "cron" {
				jobs = append(jobs, Job{
					Service: s.Name,
					Name:    strings.TrimSuffix(keyPath, ".cron"),
					Cron:    value,
					Source:  name,
				})
				seen[value] = true
			}
		})
	}

	// Java annotations add jobs whose cron is a literal, and resolve
	// ${scheduler.x.cron} references back to the yml values.
	src := filepath.Join(s.Path, "src", "main", "java")
	_ = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".java") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "@Dept44Scheduled") {
			return nil
		}
		for _, m := range dept44ScheduledRe.FindAllStringSubmatch(string(data), -1) {
			attrs := map[string]string{}
			for _, attr := range annotationAttrRe.FindAllStringSubmatch(m[1], -1) {
				attrs[attr[1]] = attr[2]
			}
			cron := attrs["cron"]
			if ref := placeholderRe.FindStringSubmatch(cron); ref != nil {
				if resolved, ok := values[ref[1]]; ok {
					cron = resolved
				}
			}
			if cron == "" || seen[cron] {
				continue
			}
			seen[cron] = true
			name := attrs["name"]
			if name == "" {
				name = strings.TrimSuffix(d.Name(), ".java")
			}
			jobs = append(jobs, Job{Service: s.Name, Name: name, Cron: cron, Source: relOrBase(s.Path, path)})
		}
		return nil
	})

	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
	return jobs
}

// ResilienceEdge is the timeout/circuit-breaker posture of one integration.
type ResilienceEdge struct {
	Service        string `json:"service"`
	Integration    string `json:"integration"`
	ConnectTimeout string `json:"connect_timeout,omitempty"`
	ReadTimeout    string `json:"read_timeout,omitempty"`
	CircuitBreaker bool   `json:"circuit_breaker"`
	MissingTimeout bool   `json:"missing_timeout"`
}

// LoadResilience reports timeout settings per integration edge of a service.
func LoadResilience(s *catalog.Service) []ResilienceEdge {
	var edges []ResilienceEdge
	for _, name := range []string{"application.yml", "application.yaml"} {
		doc := loadYaml(filepath.Join(s.Path, "src", "main", "resources", name))
		if doc == nil {
			continue
		}
		integration, _ := doc["integration"].(map[string]any)
		breakers := circuitBreakerNames(doc)
		for key, raw := range integration {
			sub, _ := raw.(map[string]any)
			edge := ResilienceEdge{Service: s.Name, Integration: key}
			for k, v := range flattenStrings(sub) {
				switch catalog.Normalize(lastSegment(k)) {
				case "connecttimeout", "connecttimeoutinseconds":
					edge.ConnectTimeout = v
				case "readtimeout", "readtimeoutinseconds":
					edge.ReadTimeout = v
				}
			}
			edge.CircuitBreaker = breakers[catalog.Normalize(key)]
			edge.MissingTimeout = edge.ConnectTimeout == "" && edge.ReadTimeout == ""
			edges = append(edges, edge)
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].Integration < edges[j].Integration })
	return edges
}

func circuitBreakerNames(doc map[string]any) map[string]bool {
	names := map[string]bool{}
	r4j, _ := doc["resilience4j"].(map[string]any)
	cb, _ := r4j["circuitbreaker"].(map[string]any)
	instances, _ := cb["instances"].(map[string]any)
	for name := range instances {
		names[catalog.Normalize(name)] = true
	}
	return names
}

// --- shared yml helpers ---

func loadYaml(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return nil
	}
	return doc
}

// walkStrings visits every string leaf with its dotted key path.
func walkStrings(node any, prefix string, visit func(keyPath, value string)) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			childPath := k
			if prefix != "" {
				childPath = prefix + "." + k
			}
			walkStrings(child, childPath, visit)
		}
	case []any:
		for _, child := range v {
			walkStrings(child, prefix, visit)
		}
	case string:
		visit(prefix, v)
	}
}

func flattenStrings(node map[string]any) map[string]string {
	out := map[string]string{}
	walkStrings(node, "", func(keyPath, value string) { out[keyPath] = value })
	// Numeric leaves matter too (timeouts are often ints).
	var walkAll func(n any, prefix string)
	walkAll = func(n any, prefix string) {
		if m, ok := n.(map[string]any); ok {
			for k, child := range m {
				childPath := k
				if prefix != "" {
					childPath = prefix + "." + k
				}
				walkAll(child, childPath)
			}
			return
		}
		if _, isString := n.(string); !isString && n != nil {
			out[prefix] = strings.TrimSpace(yamlScalar(n))
		}
	}
	walkAll(node, "")
	return out
}

func yamlScalar(v any) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func lastSegment(keyPath string) string {
	if i := strings.LastIndex(keyPath, "."); i >= 0 {
		return keyPath[i+1:]
	}
	return keyPath
}

func relOrBase(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(path)
}
