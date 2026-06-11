// Package mcpserver exposes the catalogue as MCP tools over stdio.
package mcpserver

import (
	"context"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CheeziCrew/greve/internal/catalog"
	"github.com/CheeziCrew/greve/internal/scan"
)

// maxAge is how stale the catalogue may get before a tool call triggers a
// transparent rescan. A scan is sub-second, so this is cheap insurance for
// long-lived sessions.
const maxAge = 5 * time.Minute

// server holds the current catalogue and rescans on demand.
type server struct {
	root    string
	aliases map[string]string
	orgs    []string

	mu      sync.Mutex
	catalog *catalog.Catalog
}

// Run scans once and serves the MCP tools on stdio until the client hangs up.
func Run(ctx context.Context, root string, aliases map[string]string, orgs []string) error {
	s := &server{root: root, aliases: aliases, orgs: orgs}
	if _, err := s.rescan(); err != nil {
		return err
	}

	impl := mcp.NewServer(&mcp.Implementation{Name: "greve", Version: "0.2.0"}, nil)
	s.addTools(impl)
	s.addImpactTools(impl)
	s.addOpsTools(impl)
	s.addAgentTools(impl)
	s.addFleetTools(impl)
	return impl.Run(ctx, &mcp.StdioTransport{})
}

func (s *server) rescan() (*catalog.Catalog, error) {
	services, err := scan.Scan(s.root)
	if err != nil {
		return nil, err
	}
	c := catalog.Build(s.root, services, s.aliases)
	s.mu.Lock()
	s.catalog = c
	s.mu.Unlock()
	return c, nil
}

// current returns the catalogue, rescanning first if it has gone stale.
func (s *server) current() *catalog.Catalog {
	s.mu.Lock()
	c := s.catalog
	s.mu.Unlock()
	if time.Since(c.GeneratedAt) > maxAge {
		if fresh, err := s.rescan(); err == nil {
			return fresh
		}
	}
	return c
}
