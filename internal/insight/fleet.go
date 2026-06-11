package insight

import (
	"sort"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// standardWorkflows is the org CI template every service repo should carry.
var standardWorkflows = []string{"build-and-push.yml", "maven_ci.yml"}

// FleetReport is the one-shot health overview of the landscape.
type FleetReport struct {
	Services           int            `json:"services"`
	ParentVersions     map[string]int `json:"parent_versions"`
	MissingDescription []string       `json:"missing_description"`
	PlaceholderReadme  []string       `json:"placeholder_readme"`
	MissingOwners      []string       `json:"missing_owners"`
	MissingCI          []string       `json:"missing_ci"` // lacks any standard workflow
	MissingSpec        []string       `json:"missing_spec"`
	StaleClients       int            `json:"stale_clients"`
	UnresolvedExternal []string       `json:"unresolved_external"`
	WithDatabase       int            `json:"with_database"`
	WithScheduler      int            `json:"with_scheduler"`
}

const placeholderDescription = "A concise description of what this Spring Boot microservice does."

// BuildFleetReport aggregates landscape health from the catalog plus the
// stale-client analysis.
func BuildFleetReport(c *catalog.Catalog) *FleetReport {
	report := &FleetReport{
		Services:           len(c.Services),
		ParentVersions:     map[string]int{},
		UnresolvedExternal: c.External,
	}

	for i := range c.Services {
		s := &c.Services[i]
		report.ParentVersions[s.Dept44Parent]++
		switch {
		case s.Description == "":
			report.MissingDescription = append(report.MissingDescription, s.Name)
		case s.Description == placeholderDescription:
			report.PlaceholderReadme = append(report.PlaceholderReadme, s.Name)
		}
		if len(s.Owners) == 0 {
			report.MissingOwners = append(report.MissingOwners, s.Name)
		}
		if !hasStandardWorkflow(s.Workflows) {
			report.MissingCI = append(report.MissingCI, s.Name)
		}
		if s.SpecPath == "" {
			report.MissingSpec = append(report.MissingSpec, s.Name)
		}
		if s.UsesDatabase {
			report.WithDatabase++
		}
		if s.UsesScheduler {
			report.WithScheduler++
		}
	}

	for _, hit := range StaleClients(c, "") {
		if hit.Stale {
			report.StaleClients++
		}
	}

	sort.Strings(report.MissingDescription)
	sort.Strings(report.PlaceholderReadme)
	sort.Strings(report.MissingOwners)
	sort.Strings(report.MissingCI)
	sort.Strings(report.MissingSpec)
	return report
}

func hasStandardWorkflow(workflows []string) bool {
	for _, workflow := range workflows {
		for _, standard := range standardWorkflows {
			if workflow == standard {
				return true
			}
		}
	}
	return false
}
