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

// parseClientCalls pulls the declared routes (verb + path, FeignClient base
// path prefixed) out of a client interface. These feed the broken-route check.
func TestParseClientCalls(t *testing.T) {
	src := []byte(`
@FeignClient(name = "cm", url = "${integration.care-management.url}", path = "/base")
interface CareManagementClient {
	@PostMapping(path = "/{municipalityId}/errands", consumes = APPLICATION_JSON_VALUE)
	void create();

	@GetMapping("/{municipalityId}/errands/{errandId}")
	String get();

	@RequestMapping(method = RequestMethod.DELETE, path = "/{municipalityId}/errands/{errandId}")
	void remove();
}`)
	calls := parseClientCalls(src)
	want := []ClientCall{
		{Method: "POST", Path: "/base/{municipalityId}/errands"},
		{Method: "GET", Path: "/base/{municipalityId}/errands/{errandId}"},
		{Method: "DELETE", Path: "/base/{municipalityId}/errands/{errandId}"},
	}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %+v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call %d = %+v, want %+v", i, calls[i], w)
		}
	}
}

// A Feign client in a Maven sub-module (operaton's reactor layout) outside any
// package literally named integration/ must still be found — this was the blind
// spot behind the careM↔operaton path drift going unflagged.
func TestFindFeignClientsMultiModule(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "repos", "api-service-reactor")
	clients := FindFeignClients(repo)
	if len(clients) != 1 {
		t.Fatalf("found %d clients, want 1", len(clients))
	}
	if clients[0].ConfigKey != "care-management" {
		t.Errorf("configKey = %q, want care-management", clients[0].ConfigKey)
	}
	if len(clients[0].Calls) != 1 {
		t.Errorf("calls = %d, want 1", len(clients[0].Calls))
	}
}
