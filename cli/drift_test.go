package cli

import (
	"testing"
	"time"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// material encodes the policy: a feature branch is normal here and not itself
// a problem, because parent versions are resolved against origin anyway. These
// tests exist so that policy cannot drift silently.
func TestMaterial(t *testing.T) {
	cases := []struct {
		name  string
		state catalog.GitState
		want  bool
	}{
		{
			"verified and matching origin is not material",
			catalog.GitState{OriginVerified: true},
			false,
		},
		{
			"feature branch alone is not material once verified",
			catalog.GitState{Branch: "drakel", DefaultBranch: "main", OriginVerified: true},
			false,
		},
		{
			"worktree parent differing from origin is material",
			catalog.GitState{OriginVerified: true, WorktreeParent: "8.0.9"},
			true,
		},
		{
			"unverified against origin is material — unknown is worse than known-stale",
			catalog.GitState{Branch: "main", DefaultBranch: "main"},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := material(tc.state); got != tc.want {
				t.Errorf("material(%+v) = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestStaleFetch(t *testing.T) {
	if !staleFetch(time.Time{}) {
		t.Error("an unknown fetch time must count as stale, never as fresh")
	}
	if !staleFetch(time.Now().Add(-30 * 24 * time.Hour)) {
		t.Error("30 days old must be stale")
	}
	if staleFetch(time.Now().Add(-time.Hour)) {
		t.Error("an hour old must not be stale")
	}
}

func TestFetchAge(t *testing.T) {
	if got := fetchAge(time.Time{}); got != "never" {
		t.Errorf("zero time = %q, want never", got)
	}
	if got := fetchAge(time.Now()); got != "today" {
		t.Errorf("now = %q, want today", got)
	}
	if got := fetchAge(time.Now().Add(-72 * time.Hour)); got != "3d" {
		t.Errorf("3 days = %q, want 3d", got)
	}
}
