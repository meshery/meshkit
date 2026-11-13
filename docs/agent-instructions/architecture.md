# MeshKit Architecture

MeshKit is primarily a shared Go library for the Meshery ecosystem, not a single
application. Most top-level directories are reusable packages; the executable entrypoints
live under `cmd/`, notably `cmd/errorutil` and `cmd/syncmodutil`.

Go toolchain: `go.mod` pins `go 1.26.4`; CI builds with Go 1.26 (see
`.github/workflows/ci.yml`, which also uses `go-version-file: go.mod`).

## Core pipeline 1: structured errors

The structured error pipeline spans several packages:

- `errors/` defines the shared MeshKit error types (`Error`, `ErrorV2`) and the
  `New(...)` / `NewV2(...)` / `NewV3(...)` constructors used across the entire Meshery code base
  (`NewV3` adds nested causes and remediations, see [errors.md](errors.md)).
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
- `utils/kubernetes/` builds Kubernetes and Helm clients from a kubeconfig through a
  `RESTClientGetter` that keeps exec credential plugins working (client-go runs them).
  Known cloud plugins (`aws eks get-token`, `aws-iam-authenticator`,
  `gke-gcloud-auth-plugin --use_application_default_credentials`) are instead replaced
  by in-process token minting in `utils/kubernetes/cloudauth`, the only package that
  imports the AWS and Google SDKs. Interception is fenced on purpose, because a token
  carries the Meshery process's own cloud identity and kubeconfigs often come from
  someone else: it is skipped when the config already has a token, basic auth, or a
  client certificate (client-go precedence), when the stanza selects its own identity
  (static AWS keys in exec env, the gcloud CLI account, impersonation), and when the
  server is not an endpoint of that cloud (EKS `*.eks.amazonaws.com[.cn]` or
  the IPv6 form `*.api.aws` with a matching region; GKE `*.gke.goog` or
  Connect Gateway; always https, TLS verified, no `proxy-url`). A GKE IP
  endpoint is accepted only when `container.clusters.get` or
  `container.clusters.list` shows that same IP and the kubeconfig CA (PEM) on
  a cluster in a trusted project. Trusted projects are only the ADC project
  and the process's `GOOGLE_CLOUD_PROJECT` / `GOOGLE_PROJECT`; the kubeconfig
  cannot add one. The `gke_<project>_<location>_<cluster>` context or cluster
  name is only a hint for the get call, used when its project is trusted and
  never queried otherwise; every such name seen for the same IP and CA is
  kept, so a later kubeconfig's name is used from the next lookup. A list
  with `missingZones` and no match is a failed lookup. A refused endpoint is
  not looked up again for about five minutes per IP and CA, with or without
  the plugin on PATH, so an IAM, project or hint fix takes effect after that
  window or a process restart. There is no static GKE IP list:
  Google's published cloud ranges cover every customer prefix, they change
  often, and private control planes use customer addresses. The endpoint is
  refused without calling the Container API when the kubeconfig CA cannot be
  read or parsed as PEM or no project is trusted; the lookup stops and
  refuses when a request cannot be made, the API is unreachable or times
  out, a response body cannot be read, or a 200 response is not valid JSON;
  otherwise it refuses when no get or list call
  (whatever 401, 403 or 5xx answers the others gave) returns a cluster
  matching the IP and CA. A refused endpoint uses the exec plugin when it is
  on PATH and returns `ErrUntrustedClusterEndpoint` otherwise. A confirmed IP endpoint logs `ErrInProcessGKEIPMinting` once
  per setup. The first token is minted when the kubeconfig is loaded or the
  client is built, then reused until near expiry or a 401; it is cached per
  cluster identity, and per verified endpoint for GKE IP.
  `MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true` disables minting for the
  process. The same name on a kubeconfig user's exec env disables it for
  every context that uses that user (`SetInProcessCloudAuthForContext` writes
  it on the context's AuthInfo, so contexts sharing a user switch together). No lock is held across
  network I/O; setups and mints are single-flight and deadline-bounded. The
  package doc covers the credential chains, the endpoint rules, the API
  lookup, the fallback to the plugin binary, and both disable switches.
- `broker/` provides messaging abstractions with NATS and in-process channel backends;
  the event-streaming primitives shared with Meshery Server are documented in
  [../event-streaming.md](../event-streaming.md).
- `generators/` builds MeshModel models/components from upstream sources (Artifact Hub,
  GitHub); `registry/` holds spreadsheet-driven registry tooling; `files/` handles file
  identification, sanitization, and conversion; `utils/walker/` crawls remote repositories
  for import and is documented in [../git-walkers.md](../git-walkers.md); `config/`, `schema/`, `schemas/`,
  `validator/`, `orchestration/`, and `encoding/` round out the shared surface.

## Executable entrypoints (`cmd/`)

- `cmd/errorutil` - the error-code utility described above.
- `cmd/syncmodutil` - synchronizes a destination `go.mod` with a source `go.mod`
  (rewrites shared `require` versions and appends a pinning `replace` block) so Go
  plugin builds link against identical dependency versions. See `cmd/syncmodutil/README.md`
  for behavior, the `--err` flag, and caveats.
