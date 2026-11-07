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
- Use `errors.NewV3(code, severity, short, long, []errors.Cause{...})` when an error has
  causes that each need their own remediations (meshery/meshkit#1138, the schemas error
  construct of meshery/schemas#1334). It returns `*errors.ErrorV2` with `Causes` set and
  the flat `ProbableCause` / `SuggestedRemediation` derived from them, so `GetCause`,
  `GetRemedy`, `Error()` and every existing reader keep working. See "NewV3 rules" below.
- Keep static text in the short-description, long-description, probable-cause, and
  remediation arrays as string literals, not composed constants or concatenations -
  errorutil extracts this text statically.
- Errors are namespaced to Meshery components (a component usually corresponds to one
  git repository). Codes must be unique within a component. There are no predefined
  code ranges, and codes carry no meaning (unlike HTTP status codes).

### NewV3 rules

- Write every `errors.Cause` and `errors.Remediation` as a keyed composite literal whose
  `ID`, `Description` and `Action` are string literals and whose `ActionPolicy` is
  `errors.Automatic`, `errors.RequiresApproval` or `errors.Manual` (empty means `Manual`).
- Ids are lowercase words joined by hyphens (`cluster-unreachable`) and never change once
  published: events name matched causes by id, and docs pages anchor on them. Cause ids
  are unique within an error; every cause has at least one remediation.
- A remediation that answers several causes is declared once as a package-level
  `var remX = errors.Remediation{...}` (any file of the package) and referenced from each
  cause. errorutil resolves the variable; the flat remediation list carries it once.
- `Automatic` and `RequiresApproval` need an `Action` (dot-separated lowercase words such
  as `connection.reconnect`). An unknown policy is never coerced: `errors.ValidateCauses`
  and errorutil reject it, and `Remediation.EffectivePolicy()` treats it as `Manual`.
- Every element of the short- and long-description lists is a string literal; errorutil
  reports any other element. Append instance text such as `err.Error()` to the returned
  error's `LongDescription` after `NewV3` returns.
- Unit-test a package's NewV3 definitions with `errors.ValidateCauses(errors.GetCauses(err))`.
- Go types: `ActionPolicy` and its constants alias the generated
  `github.com/meshery/schemas/models/v1beta3/errorcode` values. `Cause` and `Remediation`
  are MeshKit source types (value fields, because errorutil reads string literals and a
  pointer field cannot be one) whose JSON is exactly the schemas `ProbableCause` /
  `Remediation`; `CausesToSchema` converts them.
- MeshKit carries the nested form on three surfaces: `errors.GetCauses` and
  `errors.AsErrorV2` expose it; the structured logger emits it as a `probable-causes`
  field (nested JSON) for NewV3 errors; and `events.NewEventError` / `WithError` put it
  in an event's `metadata.error` as `probableCauses` (see
  [event-streaming](../event-streaming.md)). HTTP error responses (Meshery Server's
  `writeMeshkitError`, the schemas error response envelope) and other consumers carry
  only the flat lists today and adopt the nested form separately.

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
- errorutil reads definitions made with `errors.New`, `errors.NewV2` and `errors.NewV3`.
- errorutil emits `errorutil_analyze_errors.json` and `errorutil_analyze_summary.json`
  (analysis results: duplicates, violations of the conventions above, and
  `invalid_definitions` - NewV3 definitions it rejects, each also logged as an
  error) plus two exports. A rejected NewV3 definition (for example a non-literal
  description element, a description or causes argument that is a variable or `nil`
  instead of a literal, or one remediation id given conflicting fields by two causes) is
  left out of both exports, never exported in a reduced form, and `analyze` and `update`
  exit non-zero with an error naming each one and its problems. New and NewV2
  definitions are unaffected:
  - `errorutil_errors_export.json` - the legacy export consumed by the docs pipeline.
    Its shape is frozen (snake_case keys, joined strings) because Meshery's
    `error-codes-updater` builds `errorutil@master` and its Hugo shortcodes read those
    keys. NewV3 entries additionally carry `probableCauses` (schemas `ProbableCause[]`).
  - `errorutil_errors_index.json` - the component's definitions as a schemas
    `ErrorReferenceIndex` (`schemaVersion` 2, `ErrorDefinition` entries in code order:
    `code` `<component>-<n>`, `moniker`, `component`, `numericCode`, severity, descriptions
    joined with `. `, nested causes, derived flat lists). Every entry is validated against
    the construct's JSON Schema embedded in `github.com/meshery/schemas` before writing;
    `referenceUrl` is left for the docs build to assign.
- All four output files are git-ignored.
- The `error-codes-updater` GitHub workflow (`.github/workflows/error-codes-updater.yaml`)
  runs the update on CI and publishes `errorutil_errors_export.json` into
  `meshery/meshery`'s `docs/data/errorref/` - that is how the public error-code
  reference is generated from this repo's error registries.

## Downstream ownership

MeshKit owns shared errors, logging, and common utilities for the whole ecosystem.
Downstream repos (Meshery Server, Meshery Cloud, adapters, operators) must consume them
from MeshKit, not duplicate them locally. Conversely, shared data and API contracts
belong in `meshery/schemas`, not in MeshKit.
