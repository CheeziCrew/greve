package scan

import (
	"path/filepath"
	"testing"
)

// Classic dept44 layout (alpha): integration/betaray/BetaRayClient.java.
// Spring Modulith layout (gamma): betaray/integration/BetarayClient.java.
// Both must be found — caremanagement's modulith layout once yielded a false
// "feign_clients: 0" in integration_consistency.
func TestFindFeignClientsBothLayouts(t *testing.T) {
	cases := []struct {
		repo, pkg, configKey string
	}{
		{"api-service-alpha", "betaray", "beta-ray"},
		{"api-service-gamma", "betaray", "betaray"},
	}
	for _, c := range cases {
		repo := filepath.Join("..", "..", "testdata", "repos", c.repo)
		clients := FindFeignClients(repo)
		if len(clients) != 1 {
			t.Errorf("%s: found %d clients, want 1", c.repo, len(clients))
			continue
		}
		if clients[0].Package != c.pkg {
			t.Errorf("%s: package = %q, want %q", c.repo, clients[0].Package, c.pkg)
		}
		if clients[0].ConfigKey != c.configKey {
			t.Errorf("%s: configKey = %q, want %q", c.repo, clients[0].ConfigKey, c.configKey)
		}
	}
}

func TestFindFeignClientsNone(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "repos", "api-service-beta-ray")
	if clients := FindFeignClients(repo); len(clients) != 0 {
		t.Errorf("beta-ray: found %d clients, want 0", len(clients))
	}
}
