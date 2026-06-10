# greve

A catalogue of the Sundsvall dept44 microservices, served as a CLI and an MCP
server from one binary. Greve scans the locally cloned repos (no index files,
no daemons — a full scan takes well under a second) and answers questions like:

- *What does repo X do?* — README + OpenAPI metadata per service
- *Who talks to whom?* — call graph derived from `application*.yml` integration
  keys, `integrations/*.yaml` client specs, and pom `<inputSpec>` entries
- *Who is still on dept44 8.0.5?* — dependency/parent versions across all repos

Named after Grevé — and *greve* (count), because it counts your services.

## Install

```sh
go install github.com/CheeziCrew/greve@latest
```

## CLI

```sh
greve services [--query q] [--uses artifactId] [--db] [--scheduler] [--org x]
greve service <name>              # fuzzy: citizen / api-service-citizen both work
greve graph [name] [--direction in|out|both] [--depth n] [--dot]
greve endpoints <query> [--method POST]
greve deps <artifactId> [--version 8.0]   # dept44-service-parent works too
greve unresolved                  # integration names with no local repo
greve export [--format json|markdown] [--out file]   # deterministic CI artifact
greve github [--refresh] [--all]  # org repos vs local clones (needs gh)
greve mcp                         # serve the catalogue over MCP stdio
```

Every command takes `--json` for machine-readable output and `--root` to
point somewhere other than `~/Code/scit`.

## MCP server

```sh
claude mcp add --scope user greve -- greve mcp
```

Tools: `list_services`, `get_service`, `service_graph`, `search_endpoints`,
`dependency_versions`, `github_overview`, `refresh_catalog`. The server
rescans automatically when the catalogue is older than five minutes.

## Config (optional)

`~/.config/greve/config.yml`:

```yaml
root: ~/Code/scit
orgs: [Sundsvallskommun, Public-Service-as-a-Service]
aliases:
  # integration name -> repo dir, for names that defeat normalization
  some-weird-name: api-service-actual-repo
```

Run `greve unresolved` to see which integration names didn't resolve; anything
in that list that *is* a local repo belongs in `aliases`.

## How services are found

A direct child of the root counts as a service when its `pom.xml` declares
`se.sundsvall.dept44:dept44-service-parent` as parent. That covers all
`api-service-*` repos plus the `pw-*` process wrappers, facades, and
templates. The OpenAPI spec is searched in the known locations (main, test,
integration-test resources) preferring main; `target/`, `bin/`, and
`.claude/` are never searched.

GitHub data (repos not cloned, archived upstream) comes from the `gh` CLI and
is cached 24h in `~/Library/Caches/greve/`. Everything else works offline.

## Development

```sh
go test ./...                      # golden-file + unit tests
go test ./internal/scan -update    # regenerate the golden catalog
```
