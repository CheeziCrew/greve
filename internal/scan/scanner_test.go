package scan

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CheeziCrew/greve/internal/catalog"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestScanGolden(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "repos")

	services, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := catalog.Build(root, services, nil)

	// Normalize fields that vary between machines/runs.
	c.GeneratedAt = time.Time{}
	c.Root = "testdata/repos"
	for i := range c.Services {
		c.Services[i].Path = filepath.ToSlash(filepath.Join("testdata/repos", c.Services[i].Name))
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
	root := filepath.Join("..", "..", "testdata", "repos")
	services, err := Scan(root)
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
