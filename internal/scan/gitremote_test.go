package scan

import "testing"

func TestNormalizeRemote(t *testing.T) {
	cases := []struct {
		raw, url, org string
	}{
		{"git@github.com:Sundsvallskommun/api-service-messaging.git", "https://github.com/Sundsvallskommun/api-service-messaging", "Sundsvallskommun"},
		{"git@github.com:Public-Service-as-a-Service/api-service-citizen.git", "https://github.com/Public-Service-as-a-Service/api-service-citizen", "Public-Service-as-a-Service"},
		{"https://github.com/Sundsvallskommun/dept44.git", "https://github.com/Sundsvallskommun/dept44", "Sundsvallskommun"},
		{"https://example.com/some/repo.git", "https://example.com/some/repo", ""},
	}
	for _, c := range cases {
		url, org := normalizeRemote(c.raw)
		if url != c.url || org != c.org {
			t.Errorf("normalizeRemote(%q) = (%q, %q), want (%q, %q)", c.raw, url, org, c.url, c.org)
		}
	}
}
