package github

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ttl is how long the cached org listing stays fresh.
const ttl = 24 * time.Hour

// Cache is the persisted GitHub org listing.
type Cache struct {
	FetchedAt time.Time `json:"fetched_at"`
	Repos     []Repo    `json:"repos"`
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "greve", "github.json"), nil
}

// Load returns the org repo listing, from cache when fresh, fetching via gh
// otherwise. refresh forces a fetch. A stale cache is still returned when
// fetching fails (offline), with the error alongside.
func Load(orgs []string, refresh bool) (*Cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}

	var cached *Cache
	if data, err := os.ReadFile(path); err == nil {
		var c Cache
		if json.Unmarshal(data, &c) == nil {
			cached = &c
		}
	}
	if cached != nil && !refresh && time.Since(cached.FetchedAt) < ttl {
		return cached, nil
	}

	fresh, err := fetchAll(orgs)
	if err != nil {
		if cached != nil {
			return cached, err // stale beats nothing when offline
		}
		return nil, err
	}

	if data, err := json.MarshalIndent(fresh, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
	}
	return fresh, nil
}

func fetchAll(orgs []string) (*Cache, error) {
	c := &Cache{FetchedAt: time.Now().UTC()}
	for _, org := range orgs {
		repos, err := fetchOrg(org)
		if err != nil {
			return nil, err
		}
		c.Repos = append(c.Repos, repos...)
	}
	sort.Slice(c.Repos, func(i, j int) bool { return c.Repos[i].Name < c.Repos[j].Name })
	return c, nil
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
