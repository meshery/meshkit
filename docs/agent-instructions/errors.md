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

## Pre-commit validation

Run `make setup-hooks` once to install a local pre-commit hook that runs
`errorutil check` against your **staged** changes (the index) before each commit,
using `HEAD` as the baseline. It accepts `replace_me` and codes allocated by
`make errorutil` locally, and rejects integer codes outside the allocation
window recorded in `helpers/component_info.json` (see *Guarantees* below). It
validates only what is staged, not your full working tree.

The hook is a **developer convenience, not a security boundary**:

- `git commit --no-verify` bypasses it.
- It is not installed unless you run `make setup-hooks`.
- If the analysis itself cannot run (no Go toolchain, cold module cache, a parse
  error), it prints `could not analyze error codes locally - skipping` and lets
  the commit through.

PR CI runs the same `errorutil check` against the pull request's **merge-base**
and is authoritative regardless of whether the hook is installed. If CI cannot
determine the merge-base it **fails** the check rather than passing it.

`make test-hooks` runs the hook regression suite (bash; also runs in CI).
`make build-errorutil` builds a standalone `errorutil` binary.

### Manual Windows / Git Bash check

CI exercises the hook on Linux only. Case 13 of the suite uses a pass-through
`cygpath` mock and proves the branch executes, nothing more. To verify the
MSYS-to-native path conversion for real, in Git Bash (MINGW64):

```bash
make setup-hooks
mkdir -p pkgx && printf 'package pkgx\nconst ErrProbeCode = "meshkit-99999"\n' > pkgx/error.go
git add pkgx/error.go
git commit -m probe          # MUST be rejected, with a C:\... path in the message
git reset HEAD pkgx/error.go && rm -rf pkgx
```

## Artifacts and code allocation (`helpers/`)

- `helpers/component_info.json` is the required input metadata. For this repo it
  holds the component `name` (`meshkit`), `type` (`library`), and
  `next_error_code` — the counter from which errorutil allocates integer codes to
  placeholder symbols. Let errorutil move this counter: do not hand-pick codes,
  and do not hand-edit `next_error_code`.
- errorutil emits `errorutil_analyze_errors.json` and
  `errorutil_analyze_summary.json` (analysis results: duplicates, violations of
  the conventions above) plus `errorutil_errors_export.json` (the full export
  consumed by the docs pipeline).
- The `error-codes-updater` GitHub workflow
  (`.github/workflows/error-codes-updater.yaml`) runs the update on CI and
  publishes `errorutil_errors_export.json` into `meshery/meshery`'s
  `docs/data/errorref/` — that is how the public error-code reference is
  generated from this repo's error registries.

## What the validation actually guarantees

The checks are narrower than they look. Be precise about this.

**Guaranteed.** For a pull request compared against its merge-base:

- A new integer code that is **not** present in the baseline and **not** inside
  the allocation window `[baseline next_error_code, current next_error_code)` is
  rejected.
- A code whose number of uses **grows** relative to the baseline is rejected as a
  newly introduced duplicate. Pre-existing duplicates are tolerated at their
  existing count, so historical debt (meshery/meshkit#1106) does not block
  unrelated work.
- A `next_error_code` that moves **backwards** relative to the baseline is
  rejected.
- If CI cannot resolve the merge-base, the check fails rather than passing.

**Not guaranteed.**

- *Not "all hand-typed integers are rejected."* An integer inside the allocation
  window is accepted, and the window is derived from the pull request's own
  `helpers/component_info.json`. `make errorutil` is the supported way to produce
  such a code; widening the window by hand is not supported and is not
  mechanically prevented.
- *Allocation is local, not globally atomic.* Two branches forking from the same
  baseline can both allocate the same code and both pass validation
  independently. The conflict surfaces when one merges and the other is
  revalidated against the new baseline. There is no merge-time lock and no
  central allocator.
- *Codes are not globally unique.* Allocation state is per component: `meshkit`
  (`helpers/component_info.json`), `meshery-server`
  (`server/helpers/component_info.json`) and `mesheryctl`
  (`mesheryctl/helpers/component_info.json`) are independent counters, and the
  same integer legitimately appears in more than one of them. MeshKit CI
  validates MeshKit-owned codes; Meshery CI validates Meshery-owned codes.
- *Not every declaration is visible.* `analyze`/`check` only see string constants
  whose name matches `^Err[A-Z].+Code$` in non-`_test.go` `.go` files. A code
  under any other symbol name, or in a test file, is invisible to both the hook
  and CI.
- *Placeholders are only allocated in `error.go`.* `analyze`/`check` read every
  `.go` file, but `errorutil update` rewrites only files literally named
  `error.go`. A `replace_me` placed anywhere else passes validation and is then
  never allocated — it silently stays `replace_me`. Define error codes in
  `error.go`.
- *Deleting an error frees its code.* A new error may reuse the code of an error
  removed in the same change; that is indistinguishable from a rename and is
  accepted by design.

## Downstream ownership

MeshKit owns shared errors, logging, and common utilities for the whole ecosystem.
Downstream repos (Meshery Server, Meshery Cloud, adapters, operators) must consume them
from MeshKit, not duplicate them locally. Conversely, shared data and API contracts
belong in `meshery/schemas`, not in MeshKit.
