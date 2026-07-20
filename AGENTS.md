# MeshKit - Agent Instructions

MeshKit is the shared Go library of the Meshery ecosystem: structured errors, logging, database
wrappers (GORM over SQLite/Postgres), MeshModel registration and registry, encoding, converters,
broker clients, tracing, and general utilities. Nearly every ecosystem service (Meshery Server,
Meshery Cloud, adapters, operators, CLIs) depends on it, so changes fan out downstream and must be coordinated with consumers.

## Commands

- `make test` - `go test --short ./... -race -coverprofile=coverage.txt -covermode=atomic`
- `make check` - `golangci-lint run -c .golangci.yml -v ./...`
- `make tidy` - `go mod tidy`; fails if `go.mod` or `go.sum` change
- `make errorutil` - update structured error codes and export artifacts
- `make errorutil-analyze` - analyze structured errors without rewriting them
- `make build-errorutil` - build the `cmd/errorutil` binary
- Single test: `go test ./path/to/package -run TestName -count=1`

## Critical Rules

- **Structured errors**: concrete errors in each package's `error.go`; code symbols match
  `^Err[A-Z].+Code$`; create with `errors.NewV2(...)`; keep description/cause/remediation text as string literals.
  After adding or modifying errors, run `make errorutil` (reads `helpers/component_info.json`, emits artifacts under `helpers/`).
- **Ecosystem ownership**: MeshKit owns shared errors, logging, and common utilities for the ecosystem -
  downstream repos must not duplicate them. Shared data and API contracts belong in `meshery/schemas`, not here.
- **Registration pipeline**: one model definition per imported package/directory; ingestion is
  permissive (nested archives, YAML normalized to JSON) and mutates SVG fields to file paths.

## Identifier Naming

**Wire is camelCase everywhere; DB is snake_case; Go fields follow Go idiom; the ORM layer is the sole translation boundary.**

- Authoritative source: `meshery/schemas/AGENTS.md § Casing rules at a glance`
- Reader-friendly directory: <https://github.com/meshery/schemas/blob/master/docs/identifier-naming-contributor-guide.md>
- The contract is not optional; deviations block PRs via the schemas consumer-audit CI gate. On
  any conflict, schemas wins - file discrepancies as issues against `meshery/schemas`, not locally.
- `Id` (camelCase), never `ID`, in URL params, JSON tags, and TypeScript properties.
- meshkit: Go-only surface. Struct fields follow Go idiom (UserID); json tags are camelCase;
  db/gorm column mappings are snake_case. MeshKit is NOT the schema source of truth - types that
  mirror schema constructs come from `github.com/meshery/schemas`; do not redeclare them locally.

## Required on Every PR

- **Tests accompany every behavioral change.** Run every locally-runnable test
  before requesting review; never defer runnable coverage to reviewers or
  follow-up PRs.
- **Documentation accompanies every behavioral change, in both forms:**
  - External, user-facing: docs.meshery.io (source: meshery/meshery docs; the error-code reference
    at docs.meshery.io/reference/error-codes is generated from this repo's error registries) -
    update whenever the change is user-visible.
  - Internal, developer-facing: this repo's [`docs/`](docs/) - update whenever
    architecture, workflows, or contracts change.
- **Schema-aware changes**: run `cd ../schemas && make validate-schemas && make consumer-audit` before pushing.
- **Sign off every commit** (`git commit -s`).
- **No AI attribution** in commits, PR descriptions, comments, or code.

## AXI Agent Tooling

- Use the `gh-axi` CLI tool to interact with GitHub. Prefer `gh-axi` over `gh`.
- Use `chrome-devtools-axi` for browser automation (navigate, snapshot, click, fill forms, run JS, inspect console/network) in place of raw Playwright/chrome-devtools MCP for ad hoc tasks.
- Run `quota-axi` to check local agent-provider quota windows before long-running work.
- Use the `lavish` skill (`lavish-axi` CLI) to turn a plan, comparison, or report into a reviewable HTML artifact.

## Detailed Docs

- [architecture](docs/agent-instructions/architecture.md) - orientation: package map, the two core pipelines, cross-cutting packages.
- [errors](docs/agent-instructions/errors.md) - read before adding or changing any error: conventions, errorutil workflow, code allocation.
- [registration](docs/agent-instructions/registration.md) - read before touching `models/registration/` or `models/meshmodel/registry/`.
- [testing](docs/agent-instructions/testing.md) - make targets, flags, single-test forms, lint and tidy discipline.
- [naming-conventions](docs/agent-instructions/naming-conventions.md) - full identifier-naming contract and authority links.
- [event-streaming](docs/event-streaming.md) - read when working on events, broadcasters, or the `Event`/`EventBuilder` types shared with Meshery Server.

CLAUDE.md is a symlink to this file.
