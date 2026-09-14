package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FeignClient is one Feign client found in a service's source tree.
type FeignClient struct {
	Package    string `json:"package"`              // integration sub-package, e.g. "party"
	ClientFile string `json:"client_file"`          // relative to repo root
	ConfigKey  string `json:"config_key,omitempty"` // integration.X prefix referenced in url/properties
	// Companion files in the same package, relative to repo root.
	PropertiesFile    string `json:"properties_file,omitempty"`
	ConfigurationFile string `json:"configuration_file,omitempty"`
	// Endpoints the client declares (one per @*Mapping method).
	Calls []ClientCall `json:"calls,omitempty"`
}

// ClientCall is one HTTP route a Feign client declares, taken from a Spring
// mapping annotation. Method is "" for a @RequestMapping with no explicit
// RequestMethod (matches any verb).
type ClientCall struct {
	Method string `json:"method,omitempty"`
	Path   string `json:"path"`
}

var (
	feignAnnotationRe  = regexp.MustCompile(`@FeignClient\s*\(`)
	configKeyRe        = regexp.MustCompile(`\$\{integration\.([a-z0-9.-]+?)\.[a-z-]+\}`)
	propertiesPrefixRe = regexp.MustCompile(`@ConfigurationProperties\s*\(\s*(?:prefix\s*=\s*)?"integration\.([a-z0-9.-]+)"`)

	// Path prefix declared on the @FeignClient annotation itself (path = "...").
	feignPathRe = regexp.MustCompile(`@FeignClient\s*\([^)]*\bpath\s*=\s*"([^"]*)"`)
	// One Spring mapping annotation with its (optional) argument list.
	mappingRe = regexp.MustCompile(`@(Get|Post|Put|Delete|Patch|Request)Mapping\b\s*(?:\(([^)]*)\))?`)
	// path = "..." / value = "..." inside a mapping's args.
	mappingNamedPathRe = regexp.MustCompile(`(?:path|value)\s*=\s*\{?\s*"([^"]*)"`)
	// A bare leading string argument, e.g. @PostMapping("/x").
	mappingBarePathRe = regexp.MustCompile(`^\s*\{?\s*"([^"]*)"`)
	// RequestMethod.GET inside a @RequestMapping's args.
	requestMethodRe = regexp.MustCompile(`RequestMethod\.([A-Z]+)`)
)

// FindFeignClients walks src/main/java in the repo (every Maven module, so
// multi-module reactors are covered) and returns every @FeignClient interface
// with its companion properties/configuration files and the HTTP routes it
// declares. Discovery keys on the annotation, not the package name, so clients
// outside a conventional integration/ package are still found. Lazy: called at
// query time, not during the base scan.
func FindFeignClients(repoPath string) []FeignClient {
	var clients []FeignClient

	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".java") {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/src/main/java/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !feignAnnotationRe.Match(data) {
			return nil
		}

		dir := filepath.Dir(path)
		client := FeignClient{Package: packageNameFor(dir)}
		client.ClientFile = relOrSelf(repoPath, path)
		if m := configKeyRe.FindSubmatch(data); m != nil {
			client.ConfigKey = string(m[1])
		}
		client.Calls = parseClientCalls(data)
		attachCompanions(repoPath, dir, &client)
		clients = append(clients, client)
		return nil
	})

	sort.Slice(clients, func(i, j int) bool { return clients[i].ClientFile < clients[j].ClientFile })
	return clients
}

// parseClientCalls extracts the HTTP routes a Feign client interface declares
// from its Spring mapping annotations, prefixed with any @FeignClient(path=...).
func parseClientCalls(data []byte) []ClientCall {
	src := string(data)

	prefix := ""
	if m := feignPathRe.FindStringSubmatch(src); m != nil {
		prefix = m[1]
	}

	var calls []ClientCall
	for _, m := range mappingRe.FindAllStringSubmatch(src, -1) {
		anno, args := m[1], m[2]
		method := methodForAnnotation(anno, args)

		path := ""
		if pm := mappingNamedPathRe.FindStringSubmatch(args); pm != nil {
			path = pm[1]
		} else if bm := mappingBarePathRe.FindStringSubmatch(args); bm != nil {
			path = bm[1]
		}

		// A class-level @RequestMapping with neither a verb nor a path is a
		// base-path declaration, not a call — skip it.
		if anno == "Request" && method == "" && path == "" {
			continue
		}
		calls = append(calls, ClientCall{Method: method, Path: joinPath(prefix, path)})
	}
	return calls
}

func methodForAnnotation(anno, args string) string {
	switch anno {
	case "Get", "Post", "Put", "Delete", "Patch":
		return strings.ToUpper(anno)
	default: // Request
		if m := requestMethodRe.FindStringSubmatch(args); m != nil {
			return m[1]
		}
		return ""
	}
}

// joinPath combines a @FeignClient base path with a method path into a single
// leading-slash route.
func joinPath(prefix, path string) string {
	combined := strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
	combined = strings.TrimRight(combined, "/")
	if combined == "" {
		return "/"
	}
	if !strings.HasPrefix(combined, "/") {
		combined = "/" + combined
	}
	return combined
}

// packageNameFor labels a client package: the dir name in the classic
// layout (integration/party -> "party"), the parent in the modulith
// layout (party/integration -> "party"), and the dir name otherwise.
func packageNameFor(dir string) string {
	base := filepath.Base(dir)
	if base == "integration" {
		return filepath.Base(filepath.Dir(dir))
	}
	return base
}

// attachCompanions finds the properties/configuration classes next to a
// client and pulls the config key from the properties prefix when the client
// itself didn't reveal it.
func attachCompanions(repoPath, dir string, client *FeignClient) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(name, "Properties.java"):
			client.PropertiesFile = relOrSelf(repoPath, full)
			if client.ConfigKey == "" {
				if data, err := os.ReadFile(full); err == nil {
					if m := propertiesPrefixRe.FindSubmatch(data); m != nil {
						client.ConfigKey = string(m[1])
					}
				}
			}
		case strings.HasSuffix(name, "Configuration.java"):
			client.ConfigurationFile = relOrSelf(repoPath, full)
		}
	}
}

func relOrSelf(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
