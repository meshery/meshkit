# MeshKit Architecture

MeshKit is primarily a shared Go library for the Meshery ecosystem, not a single
application. Most top-level directories are reusable packages; the executable entrypoints
live under `cmd/`, notably `cmd/errorutil` and `cmd/syncmodutil`.

Go toolchain: `go.mod` pins `go 1.26.4`; CI builds with Go 1.26 (see
`.github/workflows/ci.yml`, which also uses `go-version-file: go.mod`).

## Core pipeline 1: structured errors

The structured error pipeline spans several packages:

- `errors/` defines the shared MeshKit error types (`Error`, `ErrorV2`) and the
  `New(...)` / `NewV2(...)` constructors used across the entire Meshery code base.
- Package-local `error.go` files define concrete errors for each package/component.
- `logger/` enriches log output with MeshKit error metadata such as code, severity,
  cause, and remediation.
- `cmd/errorutil` scans the repository, analyzes those definitions, updates placeholder
  codes using `helpers/component_info.json`, and writes export/analysis artifacts under
  `helpers/`. The `error-codes-updater` GitHub workflow runs it on CI and publishes the
  export into `meshery/meshery`'s docs data, which is how the error-code reference at
  docs.meshery.io/reference/error-codes stays generated from this repo.

Full conventions and workflow: [errors.md](errors.md).

## Core pipeline 2: MeshModel registration

- `models/registration/` ingests model packages from directories, YAML/JSON files,
  nested archives (zip/tar), and OCI artifacts, then assembles a `PackagingUnit`
  (one model definition plus its components, relationships, and connections).
- `models/meshmodel/registry/` persists registrants, models, components, and
  relationships through GORM-backed registry helpers (`RegistryManager`).
- Schema definitions themselves come from `github.com/meshery/schemas`; in this repo,
  focus on how MeshKit loads, normalizes, registers, and persists those entities rather
  than treating MeshKit as the schema source of truth.

Pipeline rules and caveats: [registration.md](registration.md).

## Cross-cutting packages

Intentionally reusable across Meshery services and tools:

- `database/` wraps GORM setup for SQLite and Postgres.
- `tracing/` provides OpenTelemetry initialization plus HTTP middleware and client
  transport helpers (see `tracing/README.md`).
- `logger/`, `utils/`, `encoding/`, and `converter/` provide shared support code used by
  Meshery services and tools.
- `broker/` provides messaging abstractions with NATS and in-process channel backends;
  the event-streaming primitives shared with Meshery Server are documented in
  [../event-streaming.md](../event-streaming.md).
- `generators/` builds MeshModel models/components from upstream sources (Artifact Hub,
  GitHub); `registry/` holds spreadsheet-driven registry tooling; `files/` handles file
  identification, sanitization, and conversion; `config/`, `schema/`, `schemas/`,
  `validator/`, `orchestration/`, and `encoding/` round out the shared surface.

## Executable entrypoints (`cmd/`)

- `cmd/errorutil` - the error-code utility described above.
- `cmd/syncmodutil` - synchronizes a destination `go.mod` with a source `go.mod`
  (rewrites shared `require` versions and appends a pinning `replace` block) so Go
  plugin builds link against identical dependency versions. See `cmd/syncmodutil/README.md`
  for behavior, the `--err` flag, and caveats.
