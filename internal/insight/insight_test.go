package insight

import (
	"path/filepath"
	"testing"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

func fixtureCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "repos")
	services, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return catalog.Build(root, services, nil)
}

func TestStaleClientsFixture(t *testing.T) {
	c := fixtureCatalog(t)
	hits := StaleClients(c, "")

	stale := map[string]string{}
	for _, h := range hits {
		if h.Stale {
			stale[h.Consumer] = h.VendoredVersion + "->" + h.CurrentVersion
		}
	}
	if stale["api-service-alpha"] != "1.0->3.0" {
		t.Errorf("alpha stale = %q, want 1.0->3.0", stale["api-service-alpha"])
	}
	if stale["api-service-gamma"] != "2.0->3.0" {
		t.Errorf("gamma stale = %q, want 2.0->3.0", stale["api-service-gamma"])
	}
}

func TestAnalyzeImpactFixture(t *testing.T) {
	c := fixtureCatalog(t)
	hits, err := AnalyzeImpact(c, "beta-ray", "GET", "/{municipalityId}/rays", "")
	if err != nil {
		t.Fatalf("AnalyzeImpact: %v", err)
	}

	byConsumer := map[string]string{}
	for _, h := range hits {
		byConsumer[h.Consumer] = h.Affected
	}
	// Both consumers vendor beta-ray specs with empty paths: checked, not affected.
	for _, consumer := range []string{"api-service-alpha", "api-service-gamma"} {
		if byConsumer[consumer] != "no" {
			t.Errorf("%s affected = %q, want no", consumer, byConsumer[consumer])
		}
	}
}

func TestAnalyzeImpactRequiresTarget(t *testing.T) {
	c := fixtureCatalog(t)
	if _, err := AnalyzeImpact(c, "beta-ray", "", "", ""); err == nil {
		t.Error("expected error when neither schema nor method+path given")
	}
}
