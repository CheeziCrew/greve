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

Catalogue:

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

Impact (integration copilot):

```sh
greve schema <service> <METHOD> <path|operationId>   # resolved request/response schema
greve impact <service> --method POST --path /x       # who breaks if this changes
greve stale [provider] [--all]    # vendored client specs behind the provider
greve consistency [service]       # feign vs yml vs vendored specs vs pom drift
greve example <provider> [--consumer name]           # real client code from a consumer
```

Ops:

```sh
greve db <service> [--history]    # tables from Flyway migrations
greve config <service>            # env vars / deploy checklist
greve jobs [service]              # cron inventory (yml + @Dept44Scheduled)
greve resilience [service]        # timeouts + circuit breakers per edge
```

Agent grounding:

```sh
greve coverage <service>          # IT suites, scenarios, covered integrations
greve patterns <type>             # real examples: scheduler|feign|validator|apptest|mapper|resource|entity
greve pack <service>              # compact markdown context card
```

Fleet:

```sh
greve activity [service]          # branches, last commit, staleness
greve search-config <query> [--keys]   # grep all application*.yml
greve path <from> <to>            # shortest call chain between services
greve fleet                       # landscape health overview
```

Review robot (local pre-commit linter + convention rulebook):

```sh
greve review <service> [--changed --base main]   # deterministic dept44 convention linter; exits 1 on errors
greve review <service> --install-hook            # local, uncommitted .git/hooks/pre-push that runs it
greve standards [category] [--file path]         # the convention rulebook (baseline + mined)
greve mine-reviews [--repos o/n,...] [--max-prs-per-repo N] [--resume]   # mine PR review comments
```

`greve review` is a local gate only — it never touches CI, the Maven build, or
GitHub. It flags lexical conventions the build and SonarCloud don't: ternaries,
missing `{Resource}FailureTest`, missing `@CircuitBreaker`, Lombok,
`org.zalando.problem` imports, enums in `api/model`, `@Scheduled` vs
`@Dept44Scheduled`, field injection, non-static-imported `HttpStatus`. Errors
exit non-zero; warnings advise. Disable/retune rules per repo via an
(uncommitted) `.greve-review.yml`.

`greve standards` serves a rulebook distilled from CLAUDE.md + the pattern files
(embedded baseline) merged with any mined override at
`<UserConfigDir>/greve/standards-corpus.json`. `greve mine-reviews` populates
that pipeline: it pulls human PR review comments via `gh` GraphQL into
`~/Library/Caches/greve/reviews.ndjson` (bot/noise filtered, resumable,
deduped); a Claude Workflow then distils them into the override greve serves.

Every command takes `--json` for machine-readable output and `--root` to
point somewhere other than `~/Code/scit`.

## MCP server

```sh
claude mcp add --scope user greve -- greve mcp
```

Or, inside Sundsvalls kommun, install the `greve` plugin from the
[Sundsvallskommun/claude-plugins](https://github.com/Sundsvallskommun/claude-plugins)
marketplace — it registers the MCP server, adds a `/greve:review` command and a
usage skill (the binary still has to be installed as above):

```
/plugin marketplace add Sundsvallskommun/claude-plugins
/plugin install greve@sundsvall-claude-plugins
```

26 tools mirroring the CLI: `list_services`, `get_service`, `service_graph`,
`search_endpoints`, `dependency_versions`, `github_overview`,
`refresh_catalog`, `endpoint_schema`, `impact_analysis`, `stale_clients`,
`integration_consistency`, `usage_examples`, `db_schema`, `config_surface`,
`scheduler_jobs`, `resilience_report`, `test_coverage`, `pattern_examples`,
`context_pack`, `git_activity`, `search_config`, `path_between`,
`fleet_report`, `review_diff`, `convention_rules`, `standards_for_file`. The
server rescans automatically when the catalogue is older
than five minutes. Heavy extractors (Flyway, Feign, spec resolution, git)
run lazily per query — the base scan stays sub-second.

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
