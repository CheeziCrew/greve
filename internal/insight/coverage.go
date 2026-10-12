package insight

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// ITClass is one integration-test suite with its WireMock scenarios.
type ITClass struct {
	Name      string   `json:"name"`
	Scenarios []string `json:"scenarios"`
}

// CoverageReport maps a service's integration tests to its integrations.
type CoverageReport struct {
	Service string    `json:"service"`
	Suites  []ITClass `json:"suites"`
	// CoveredIntegrations lists integrations whose name appears in stub
	// fixture paths — a heuristic, not proof of behavioral coverage.
	CoveredIntegrations   []string `json:"covered_integrations"`
	UncoveredIntegrations []string `json:"uncovered_integrations"`
}

// LoadCoverage walks src/integration-test/resources for <Name>IT suites and
// their __files scenario directories.
func LoadCoverage(s *catalog.Service) *CoverageReport {
	report := &CoverageReport{Service: s.Name}
	root := filepath.Join(s.Path, "src", "integration-test", "resources")

	entries, err := os.ReadDir(root)
	if err != nil {
		return report
	}

	var stubText strings.Builder
	for _, e := range entries {
		// Keyed on the suite having WireMock fixtures rather than on its directory
		// being named <Something>IT. @WireMockAppTestSuite(files = ...) names the
		// directory freely — api-service-supportcenter points its end-of-lease
		// suites at "EndOfLeaseDispatch/" — and requiring the IT suffix made every
		// stub in such a suite invisible, reporting its integrations as untested.
		if !e.IsDir() || !hasWireMockFixtures(filepath.Join(root, e.Name())) {
			continue
		}
		suite := ITClass{Name: e.Name(), Scenarios: []string{}}
		filesDir := filepath.Join(root, e.Name(), "__files")
		if scenarios, err := os.ReadDir(filesDir); err == nil {
			for _, scenario := range scenarios {
				if scenario.IsDir() {
					suite.Scenarios = append(suite.Scenarios, scenario.Name())
				}
			}
		}
		sort.Strings(suite.Scenarios)
		report.Suites = append(report.Suites, suite)

		// Collect every fixture path under the suite for the integration
		// name heuristic below.
		_ = filepath.WalkDir(filepath.Join(root, e.Name()), func(path string, d os.DirEntry, err error) error {
			if err == nil {
				stubText.WriteString(strings.ToLower(path) + "\n")
			}
			return nil
		})
	}
	sort.Slice(report.Suites, func(i, j int) bool { return report.Suites[i].Name < report.Suites[j].Name })

	haystack := catalog.Normalize(stubText.String())
	for _, integration := range s.Integrations {
		if strings.Contains(haystack, catalog.Normalize(integration.Name)) {
			report.CoveredIntegrations = append(report.CoveredIntegrations, integration.Name)
		} else {
			report.UncoveredIntegrations = append(report.UncoveredIntegrations, integration.Name)
		}
	}
	sort.Strings(report.CoveredIntegrations)
	sort.Strings(report.UncoveredIntegrations)
	return report
}

// hasWireMockFixtures reports whether a directory under integration-test
// resources is a WireMock suite, i.e. holds a __files or mappings subdirectory.
// Distinguishes a suite from the other things that live beside it (db/scripts,
// junit config) without relying on a naming convention.
func hasWireMockFixtures(dir string) bool {
	for _, marker := range []string{"__files", "mappings"} {
		if info, err := os.Stat(filepath.Join(dir, marker)); err == nil && info.IsDir() {
			return true
		}
	}
	// Scenario directories may carry the markers one level down.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, marker := range []string{"__files", "mappings"} {
			if info, err := os.Stat(filepath.Join(dir, e.Name(), marker)); err == nil && info.IsDir() {
				return true
			}
		}
	}
	return false
}
