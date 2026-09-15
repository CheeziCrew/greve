package scan

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/testfixtures"
)

var update = flag.Bool("update", false, "rewrite golden files")

// materializeRepos restores the fixtures' dotgit/ directories to .git/ in a
// temp dir. Scan needs a real .git to tell a service from a stray copy.
func materializeRepos(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := testfixtures.Materialize(filepath.Join("..", "..", "testdata", "repos"), dst); err != nil {
		t.Fatalf("materialize fixtures: %v", err)
	}
	return dst
}

func TestScanGolden(t *testing.T) {
	root := materializeRepos(t)

	services, nonRepos, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := catalog.Build(root, services, nil)
	c.NotRepositories = nonRepos
	for i := range c.NotRepositories {
		c.NotRepositories[i].Path = filepath.ToSlash(filepath.Join("testdata/repos", c.NotRepositories[i].Name))
	}

	// Normalize fields that vary between machines/runs.
	c.GeneratedAt = time.Time{}
	c.Root = "testdata/repos"
	for i := range c.Services {
		c.Services[i].Path = filepath.ToSlash(filepath.Join("testdata/repos", c.Services[i].Name))
		// LastFetch is the mtime of files the fixture copy just created, so it
		// differs every run and on every machine.
		c.Services[i].Git.LastFetch = time.Time{}
	}

	got, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("..", "..", "testdata", "golden", "catalog.json")
	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("catalog mismatch with golden file; run 'go test ./internal/scan -update' if the change is intended\ngot:\n%s", got)
	}
}

func TestScanFindsOnlyDept44Services(t *testing.T) {
	root := materializeRepos(t)
	services, _, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(services) != 3 {
		names := make([]string, 0, len(services))
		for _, s := range services {
			names = append(names, s.Name)
		}
		t.Fatalf("expected 3 services, got %d: %v", len(services), names)
	}
}

func TestScanExcludesNonRepositories(t *testing.T) {
	root := materializeRepos(t)

	services, nonRepos, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	for _, s := range services {
		if s.Name == "stray-no-git" {
			t.Fatal("stray-no-git has no .git and must not be counted as a service")
		}
	}
	if len(nonRepos) != 1 {
		t.Fatalf("expected 1 non-repository, got %d: %+v", len(nonRepos), nonRepos)
	}
	if nonRepos[0].Name != "stray-no-git" {
		t.Errorf("expected stray-no-git, got %q", nonRepos[0].Name)
	}
	if nonRepos[0].Reason == "" {
		t.Error("a skipped directory must carry a reason")
	}
}

// A stray directory can carry the artifactId of a live service — that is how
// api-service-operaton-old shadowed api-service-operaton. Excluding it must
// also keep it out of the lookup index.
func TestStrayDoesNotShadowRealService(t *testing.T) {
	root := materializeRepos(t)

	services, nonRepos, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := catalog.Build(root, services, nil)
	c.NotRepositories = nonRepos

	got := c.Lookup("api-service-alpha")
	if got == nil {
		t.Fatal("api-service-alpha should resolve")
	}
	if got.Name != "api-service-alpha" || got.GroupID == "se.sundsvall.stray" {
		t.Errorf("lookup resolved to the stray copy: %+v", got)
	}
}

// A repo that was git init'ed but never pushed has a .git with no origin
// remote. It is still a real repository and must be catalogued — this is the
// test that stops the .git probe being "simplified" to RepoURL != "".
func TestUnpushedRepoIsStillAService(t *testing.T) {
	root := materializeRepos(t)

	services, _, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, s := range services {
		if s.Name == "api-service-gamma" {
			if s.RepoURL != "" {
				t.Errorf("fixture has no origin remote, expected empty RepoURL, got %q", s.RepoURL)
			}
			return
		}
	}
	t.Fatal("api-service-gamma has a .git but no remote; it must still be a service")
}

func TestScanReadsOriginRemote(t *testing.T) {
	root := materializeRepos(t)

	services, _, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := map[string]string{
		"api-service-alpha":    "https://github.com/Sundsvallskommun/api-service-alpha",
		"api-service-beta-ray": "https://github.com/Sundsvallskommun/api-service-beta-ray",
	}
	for _, s := range services {
		if expected, ok := want[s.Name]; ok {
			if s.RepoURL != expected {
				t.Errorf("%s RepoURL = %q, want %q", s.Name, s.RepoURL, expected)
			}
			if s.Org != "Sundsvallskommun" {
				t.Errorf("%s Org = %q, want Sundsvallskommun", s.Name, s.Org)
			}
			delete(want, s.Name)
		}
	}
	if len(want) > 0 {
		t.Errorf("services never seen: %v", want)
	}
}

func TestIntegrationNameFromSpec(t *testing.T) {
	cases := map[string]string{
		"party-2.0.yml":                "party",
		"citizen-v3.yml":               "citizen",
		"oep-integrator-1.5.yml":       "oep-integrator",
		"digital-mail-sender-4.1.yml":  "digital-mail-sender",
		"betaray-v2.yaml":              "betaray",
		"messaging-api.yaml":           "messaging-api",
		"api-datawarehousereader.yaml": "api-datawarehousereader",
	}
	for in, want := range cases {
		if got := integrationNameFromSpec(in); got != want {
			t.Errorf("integrationNameFromSpec(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSpecDiscoveryPrefersMain(t *testing.T) {
	repo := t.TempDir()
	for _, p := range []string{
		"src/test/resources/api/openapi.yml",
		"src/main/resources/api/openapi.yaml",
	} {
		full := filepath.Join(repo, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("openapi: 3.0.1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := findSpec(repo); got != "src/main/resources/api/openapi.yaml" {
		t.Errorf("findSpec = %q, want main resources candidate", got)
	}
}

func TestSpecDiscoveryFallbackSkipsTarget(t *testing.T) {
	repo := t.TempDir()
	for _, p := range []string{
		"src/main/resources/odd-place/openapi.yaml",
		"src/main/resources/integrations/openapi.yaml",
	} {
		full := filepath.Join(repo, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("openapi: 3.0.1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// target/ pollution outside src/ is never reached; integrations/ inside
	// src/ must be skipped, leaving only the odd-place spec.
	if got := findSpec(repo); got != "src/main/resources/odd-place/openapi.yaml" {
		t.Errorf("findSpec = %q, want odd-place spec", got)
	}
}
