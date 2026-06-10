// Package scan walks a root directory of cloned repos and extracts the
// machine-readable metadata of every dept44 microservice found.
package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/CheeziCrew/greve/internal/catalog"
)

const apiServicePrefix = "api-service-"

// Scan walks the direct children of root and returns every repo whose
// pom.xml declares the dept44 service parent, sorted by name.
func Scan(root string) ([]catalog.Service, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var (
		mu       sync.Mutex
		services []catalog.Service
	)
	var group errgroup.Group
	group.SetLimit(16)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		repoPath := filepath.Join(root, name)
		group.Go(func() error {
			service, ok := scanRepo(repoPath, name)
			if !ok {
				return nil
			}
			mu.Lock()
			services = append(services, service)
			mu.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services, nil
}

func scanRepo(repoPath, name string) (catalog.Service, bool) {
	pom, err := parsePom(filepath.Join(repoPath, "pom.xml"))
	if err != nil || !pom.isDept44Service() {
		return catalog.Service{}, false
	}

	service := catalog.Service{
		Name:         name,
		ShortName:    strings.TrimPrefix(name, apiServicePrefix),
		Path:         repoPath,
		GroupID:      pom.groupID,
		ArtifactID:   pom.artifactID,
		Version:      pom.version,
		Dept44Parent: pom.parentVersion,
		Dependencies: pom.dependencies,
	}

	service.RepoURL, service.Org = gitRemote(repoPath)
	service.Description = readmeDescription(repoPath)

	if specPath := findSpec(repoPath); specPath != "" {
		if api, err := parseSpec(filepath.Join(repoPath, filepath.FromSlash(specPath))); err == nil {
			service.API = api
			service.SpecPath = specPath
			if service.Description == "" {
				service.Description = api.Title
			}
		}
	}

	cfg := parseAppConfigs(repoPath)
	service.Integrations = collectIntegrations(cfg, listIntegrationSpecs(repoPath), pom.inputSpecs)

	service.UsesScheduler = cfg.hasCron || hasDependency(pom.dependencies, "dept44-starter-scheduler")
	service.DatabaseKind = cfg.databaseKind
	if service.DatabaseKind == "" && hasDependency(pom.dependencies, "mariadb-java-client") {
		service.DatabaseKind = "mariadb"
	}
	service.UsesDatabase = service.DatabaseKind != "" ||
		hasDependency(pom.dependencies, "spring-boot-starter-data-jpa") ||
		hasDependency(pom.dependencies, "dept44-starter-jpa")

	return service, true
}

func hasDependency(deps []catalog.Dependency, artifactID string) bool {
	for _, d := range deps {
		if d.ArtifactID == artifactID {
			return true
		}
	}
	return false
}

// specVersionRe strips trailing version suffixes from integration spec
// filenames: party-2.0.yml, citizen-v3.yml, oep-integrator-1.5.yml.
var specVersionRe = regexp.MustCompile(`-v?\d+(\.\d+)*$`)

// integrationNameFromSpec derives an integration name from a spec filename.
func integrationNameFromSpec(filename string) string {
	name := filename
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		name = strings.TrimSuffix(name, ext)
	}
	return specVersionRe.ReplaceAllString(name, "")
}

// collectIntegrations unions the three signals (application.yml integration
// keys, integrations/ spec files, pom inputSpecs) into one deduplicated,
// evidence-tagged list. Entries merge on the normalized name; the yml key
// spelling wins as the canonical name since it is the cleanest.
func collectIntegrations(cfg *appConfig, specFiles, inputSpecs []string) []catalog.Integration {
	merged := map[string]*catalog.Integration{}

	add := func(name, source string, canonical bool) {
		key := catalog.Normalize(name)
		if key == "" {
			return
		}
		entry, ok := merged[key]
		if !ok {
			entry = &catalog.Integration{Name: name}
			merged[key] = entry
		}
		if canonical {
			entry.Name = name
		}
		for _, s := range entry.Sources {
			if s == source {
				return
			}
		}
		entry.Sources = append(entry.Sources, source)
	}

	for _, name := range cfg.sortedIntegrationNames() {
		for _, file := range cfg.integrationKeys[name] {
			add(name, file, true)
		}
	}
	for _, file := range specFiles {
		add(integrationNameFromSpec(file), "integrations/"+file, false)
	}
	for _, file := range inputSpecs {
		add(integrationNameFromSpec(file), "pom:inputSpec", false)
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	integrations := make([]catalog.Integration, 0, len(keys))
	for _, key := range keys {
		entry := merged[key]
		sort.Strings(entry.Sources)
		integrations = append(integrations, *entry)
	}
	return integrations
}
