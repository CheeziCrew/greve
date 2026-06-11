package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FeignClient is one Feign client found in a service's integration packages.
type FeignClient struct {
	Package    string `json:"package"`              // integration sub-package, e.g. "party"
	ClientFile string `json:"client_file"`          // relative to repo root
	ConfigKey  string `json:"config_key,omitempty"` // integration.X prefix referenced in url/properties
	// Companion files in the same package, relative to repo root.
	PropertiesFile    string `json:"properties_file,omitempty"`
	ConfigurationFile string `json:"configuration_file,omitempty"`
}

var (
	feignAnnotationRe  = regexp.MustCompile(`@FeignClient\s*\(`)
	configKeyRe        = regexp.MustCompile(`\$\{integration\.([a-z0-9.-]+?)\.[a-z-]+\}`)
	propertiesPrefixRe = regexp.MustCompile(`@ConfigurationProperties\s*\(\s*(?:prefix\s*=\s*)?"integration\.([a-z0-9.-]+)"`)
)

// FindFeignClients walks src/main/java/**/integration/ packages and returns
// every @FeignClient interface with its companion properties/configuration
// files. Lazy: called at query time, not during the base scan.
func FindFeignClients(repoPath string) []FeignClient {
	integrationDirs := findIntegrationDirs(repoPath)
	var clients []FeignClient

	for _, dir := range integrationDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".java") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(full)
			if err != nil || !feignAnnotationRe.Match(data) {
				continue
			}

			client := FeignClient{Package: filepath.Base(dir)}
			client.ClientFile = relOrSelf(repoPath, full)
			if m := configKeyRe.FindSubmatch(data); m != nil {
				client.ConfigKey = string(m[1])
			}
			attachCompanions(repoPath, dir, &client)
			clients = append(clients, client)
		}
	}

	sort.Slice(clients, func(i, j int) bool { return clients[i].ClientFile < clients[j].ClientFile })
	return clients
}

// findIntegrationDirs returns leaf package dirs under any **/integration/
// directory in src/main/java.
func findIntegrationDirs(repoPath string) []string {
	var dirs []string
	src := filepath.Join(repoPath, "src", "main", "java")
	_ = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "integration" {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
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
