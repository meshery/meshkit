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

## Claude Code Settings

- `.claude/settings.json` is **tracked** shared config: hook registrations every contributor gets
  from a fresh clone. Every `command` path in it must resolve to a script that exists - a
  registration pointing at a missing script is a guard the repo silently never runs.
- `.claude/settings.local.json` is **git-ignored** per-developer state. Claude Code rewrites it on
  every session, so tracking it produces spurious diffs and truncation. Keep MCP server toggles,
  `permissions`, and `additionalDirectories` (e.g. `../schemas`) there.
- Of the registered guards, only `no-ai-attribution.sh` is active repo-wide. `meshkit-errors.sh`
  and `guard-local-models.sh` match `server/**`, which MeshKit does not have, and `session-start.sh`
  runs only when `CLAUDE_CODE_REMOTE=true` - do not assume they enforce anything here.
  - `session-start.sh` enforces nothing, but it is not inert. When `CLAUDE_CODE_REMOTE=true` it
    overwrites your **global** git identity - `git config --global user.name` and `user.email`,
    resolved from the authenticated `gh` user - and shallow-clones sibling repos such as
    `meshery/schemas` into the **parent** directory. Decide whether you want both before it runs.
- The tracked `.claude/hookify.*.local.md` rule files are a **second, independent** guard mechanism,
  read directly by the hookify plugin and not wired through `settings.json` at all.
  `hookify.no-ai-attribution-bash.local.md` and `hookify.no-ai-attribution-file.local.md` carry no
  path filter and do fire here; `hookify.meshkit-errors.local.md` requires `file_path` matching
  `/server/.*\.go$` and is inert in MeshKit, the same dead-guard pattern as above. So three guards
  are live in this repo, not one.

**Already have this repo cloned?** `.claude/settings.local.json` used to be tracked and carried the
hook registrations that now live in `.claude/settings.json`. Migrate in exactly this order:

1. **Back up `.claude/settings.local.json` before pulling - required, not precautionary.** The file
   goes from tracked to ignored here, so the pull removes it from your working tree, and may first
   abort with a modify/delete conflict if you have local modifications (common - Claude Code
   rewrites this file constantly). Skip the backup and your MCP server selection, `permissions`,
   and `additionalDirectories` are gone for good.
2. **After pulling, restore the backup to `.claude/settings.local.json`.** The pull removed it. The
   path is git-ignored from here on, so the restored file stays local and is never tracked again.
3. **Then delete the `hooks` block from the restored file** - after the restore, not before, because
   restoring the whole backup reinstates the stale block. Claude Code merges `settings.json` and
   `settings.local.json` additively, so leaving the block in place fires every promoted guard twice,
   a symptom you will not connect to a settings change days later. Deleting the whole block is also
   what drops the dead `PostToolUse` registration pointing at `tools/hooks/helm-chart-audit.py` -
   that path does not exist in this repo, which is why the tracked file omits it.
4. **Keep everything else in the restored file**: `enabledMcpjsonServers` /
   `disabledMcpjsonServers`, `permissions`, and `additionalDirectories` (e.g. `../schemas`) are
   per-machine and belong there.

The same pull also untracks six generated `__pycache__/*.pyc` files under `.claude/` and
`.agents/`. If Python rewrote yours since you cloned, the pull aborts naming those paths - delete
the `__pycache__` directories and pull again. They are git-ignored from here on and regenerate on
next use.

## Detailed Docs

- [architecture](docs/agent-instructions/architecture.md) - orientation: package map, the two core pipelines, cross-cutting packages.
- [errors](docs/agent-instructions/errors.md) - read before adding or changing any error: conventions, errorutil workflow, code allocation.
- [registration](docs/agent-instructions/registration.md) - read before touching `models/registration/` or `models/registry/manager/`.
- [testing](docs/agent-instructions/testing.md) - make targets, flags, single-test forms, lint and tidy discipline.
- [naming-conventions](docs/agent-instructions/naming-conventions.md) - full identifier-naming contract and authority links.
- [event-streaming](docs/event-streaming.md) - read when working on events, broadcasters, or the `Event`/`EventBuilder` types shared with Meshery Server.

CLAUDE.md is a symlink to this file.
