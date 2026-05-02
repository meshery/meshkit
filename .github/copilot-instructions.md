# Copilot Instructions

## Build, test, and lint

- Use Go 1.25.x (`go.mod` pins `go 1.25.5`; CI uses Go 1.25).
- Prefer the repo `make` targets for final verification:
  - `make test` - runs `go test --short ./... -race -coverprofile=coverage.txt -covermode=atomic`
  - `make check` - runs `golangci-lint run -c .golangci.yml -v ./...`
  - `make tidy` - runs `go mod tidy` and fails if `go.mod` or `go.sum` changed
  - `make build-errorutil` - builds the `cmd/errorutil` binary
  - `make errorutil` - updates structured error codes and related exports
  - `make errorutil-analyze` - analyzes structured error definitions without rewriting them
- For a single test during iteration, run Go tests directly at package scope:
  - `go test ./models/registration -run TestGetEntityAcceptsV1Beta2RelationshipSchemaVersion -count=1`
  - general form: `go test ./path/to/package -run TestName -count=1`

## High-level architecture

- This repository is primarily a **shared Go library** for the Meshery ecosystem, not a single application. Most top-level directories are reusable packages; the main executable entrypoints live under `cmd/`, notably `cmd/errorutil` and `cmd/syncmodutil`.
- The **structured error pipeline** spans several packages:
  - `errors/` defines the shared MeshKit error types and helpers.
  - package-local `error.go` files define concrete errors for each package/component.
  - `logger/` enriches log output with MeshKit error metadata such as code, severity, cause, and remediation.
  - `cmd/errorutil` scans the repository, analyzes those definitions, updates placeholder codes using `helpers/component_info.json`, and writes export/analysis artifacts under `helpers/`.
- The **MeshModel registration pipeline** is another core slice of the repo:
  - `models/registration/` ingests model packages from directories, YAML/JSON files, nested archives, and OCI artifacts, then assembles a `PackagingUnit`.
  - `models/meshmodel/registry/` persists registrants, models, components, and relationships through GORM-backed registry helpers.
  - Schema definitions themselves come from `github.com/meshery/schemas`; in this repo, focus on how MeshKit loads, normalizes, registers, and persists those entities rather than treating MeshKit as the schema source of truth.
- Cross-cutting infrastructure packages are intentionally reusable across services:
  - `database/` wraps GORM setup for SQLite and Postgres.
  - `tracing/` provides OpenTelemetry initialization plus HTTP middleware/client transport helpers.
  - `logger/`, `utils/`, `encoding/`, and `converter/` provide shared support code used by Meshery services and tools.

## Key conventions

- Prefer **Makefile targets for final verification**. Use direct `go test ./pkg -run TestName` only for focused iteration.
- Strictly honor and adhere to the established identifier naming scheme, authoritatively documented in meshery/schemas - https://github.com/meshery/schemas/blob/master/docs/identifier-naming-contributor-guide.md#canonical-naming-directory
- Structured errors follow strict repository conventions so `errorutil` can understand them:
  - define concrete errors in each package's `error.go`
  - define code symbols matching `^Err[A-Z].+Code$`
  - create errors with `errors.New(...)` (or `errors.NewV2(...)` when needed; the `(*errors.Error).ErrorV2(...)` helper may also be used where applicable)
  - keep static text in the description/cause/remediation arrays as string literals rather than composed constants/concatenations
- If you add or modify structured errors, run `make errorutil` and usually `make errorutil-analyze`. `errorutil` expects `component_info.json` metadata and emits `errorutil_analyze_*.json` plus `errorutil_errors_export.json` artifacts under `helpers/`.
- MeshModel registration assumes an imported package/directory contains **exactly one model definition** and then any number of components/relationships associated with it.
- Registration code is intentionally permissive on input shape:
  - directory import recursively unwraps nested zip/tar/OCI content
  - YAML is normalized to JSON before entity detection/unmarshal
  - registration accepts both legacy and canonical schema-version strings for models/components/relationships where compatibility is required
- Registration mutates asset references as part of ingestion: model/component SVG fields are written to the filesystem and replaced with file paths before persistence through `RegistryManager`.
