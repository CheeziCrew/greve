package catalog

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"alk-t":        "alkt",
		"case-data":    "casedata",
		"API-Service":  "apiservice",
		"party_2.0":    "party20",
		"Lantmäteriet": "lantmteriet",
		"":             "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func testServices() []Service {
	return []Service{
		{Name: "api-service-alkt", ShortName: "alkt", ArtifactID: "api-service-alkt"},
		{Name: "api-service-messaging", ShortName: "messaging", ArtifactID: "api-service-messaging"},
		{
			Name: "api-service-caller", ShortName: "caller", ArtifactID: "api-service-caller",
			Integrations: []Integration{
				{Name: "alk-t", Sources: []string{"application.yml"}},
				{Name: "messaging-api", Sources: []string{"integrations/messaging-api.yaml"}},
				{Name: "lantmateriet", Sources: []string{"application.yml"}},
				{Name: "weird-name", Sources: []string{"application.yml"}},
			},
		},
	}
}

func TestBuildResolution(t *testing.T) {
	c := Build("root", testServices(), map[string]string{"weird-name": "api-service-messaging"})

	caller := c.Lookup("caller")
	if caller == nil {
		t.Fatal("caller not found")
	}

	want := map[string]string{
		"alk-t":         "api-service-alkt",      // hyphen variance via Normalize
		"messaging-api": "api-service-messaging", // -api suffix stripped
		"lantmateriet":  "",                      // genuinely external
		"weird-name":    "api-service-messaging", // alias map
	}
	for _, integration := range caller.Integrations {
		if got := integration.ResolvedTo; got != want[integration.Name] {
			t.Errorf("integration %q resolved to %q, want %q", integration.Name, got, want[integration.Name])
		}
	}

	if got := c.External; len(got) != 1 || got[0] != "lantmateriet" {
		t.Errorf("External = %v, want [lantmateriet]", got)
	}

	messaging := c.Lookup("messaging")
	if len(messaging.ConsumedBy) != 1 || messaging.ConsumedBy[0] != "api-service-caller" {
		t.Errorf("messaging.ConsumedBy = %v, want [api-service-caller]", messaging.ConsumedBy)
	}
}

func TestLookupFuzzy(t *testing.T) {
	c := Build("root", testServices(), nil)
	for _, name := range []string{"messaging", "api-service-messaging", "MESSAGING"} {
		if c.Lookup(name) == nil {
			t.Errorf("Lookup(%q) = nil, want api-service-messaging", name)
		}
	}
	if c.Lookup("nonexistent") != nil {
		t.Error("Lookup(nonexistent) should be nil")
	}
}
