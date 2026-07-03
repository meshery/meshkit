# Structured Errors and the errorutil Workflow

MeshKit-compatible structured errors are a repo-wide (and ecosystem-wide) convention.
Uniform definitions allow error information to be extracted directly from the code and
published automatically as the error-code reference at
docs.meshery.io/reference/error-codes. The `cmd/errorutil` tool is part of that
toolchain and only works if the conventions below are followed strictly.

## Conventions (strict - errorutil parses these)

- Define concrete errors in each package's `error.go`.
- Define error-code symbols as string constants (preferably constants, variables are
  tolerated) whose names match the regex `^Err[A-Z].+Code$`, e.g. `ErrApplyManifestCode`.
- The initial value of a new code is a placeholder string (e.g. `"replace_me"`) set by
  the developer. The final value is an integer string assigned by errorutil.
- Create errors with `errors.NewV2(...)` when no existing error is a well-suited fit;
  the `(*errors.Error).NewV2(...)` helper may also be used where applicable. The legacy
  `errors.New(...)` constructor remains for existing call sites.
- Keep static text in the short-description, long-description, probable-cause, and
  remediation arrays as string literals, not composed constants or concatenations -
  errorutil extracts this text statically.
- Errors are namespaced to Meshery components (a component usually corresponds to one
  git repository). Codes must be unique within a component. There are no predefined
  code ranges, and codes carry no meaning (unlike HTTP status codes).

See the package documentation in `errors/errors.go` and
<https://docs.meshery.io/project/contributing/contributing-error> for background.

## Workflow after adding or modifying errors

1. Run `make errorutil` - rewrites placeholder codes to real integer codes and updates
   exports. Usually also run `make errorutil-analyze` to verify without rewriting.
2. Both targets run `cmd/errorutil` as
   `go run github.com/meshery/meshkit/cmd/errorutil -d . <update|analyze> --skip-dirs meshery -i ./helpers -o ./helpers`.
3. Commit the changed source files together with the regenerated artifacts.

`make build-errorutil` builds a standalone `errorutil` binary from `cmd/errorutil/main.go`.

## Artifacts and code allocation (`helpers/`)

- `helpers/component_info.json` is the required input metadata. For this repo it holds
  the component `name` (`meshkit`), `type` (`library`), and `next_error_code` - the
  counter from which errorutil allocates integer codes to placeholder symbols. Do not
  hand-pick codes; let errorutil allocate them from `next_error_code`.
- errorutil emits `errorutil_analyze_errors.json` and `errorutil_analyze_summary.json`
  (analysis results: duplicates, violations of the conventions above) plus
  `errorutil_errors_export.json` (the full export consumed by the docs pipeline).
- The `error-codes-updater` GitHub workflow (`.github/workflows/error-codes-updater.yaml`)
  runs the update on CI and publishes `errorutil_errors_export.json` into
  `meshery/meshery`'s `docs/data/errorref/` - that is how the public error-code
  reference is generated from this repo's error registries.

## Downstream ownership

MeshKit owns shared errors, logging, and common utilities for the whole ecosystem.
Downstream repos (Meshery Server, Meshery Cloud, adapters, operators) must consume them
from MeshKit, not duplicate them locally. Conversely, shared data and API contracts
belong in `meshery/schemas`, not in MeshKit.
