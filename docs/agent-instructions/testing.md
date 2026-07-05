# Testing, Linting, and Module Hygiene

## Go toolchain

Use the Go version pinned by `go.mod` (`go 1.26.4`); CI builds with Go 1.26 and
`go-version-file: go.mod`.

## Make targets (prefer these for final verification)

- `make test` - runs `go test --short ./... -race -coverprofile=coverage.txt -covermode=atomic`.
  Note the flags: `--short` skips long-running tests (respect `testing.Short()` when
  writing slow tests) and `-race` means all tests must be race-clean.
- `make check` - runs `golangci-lint run -c .golangci.yml -v ./...`. Lint config lives
  in `.golangci.yml`; fix findings rather than suppressing them.
- `make tidy` - runs `go mod tidy` and then `git diff --exit-code go.mod go.sum`, so it
  fails if `go.mod` or `go.sum` changed. Run it after any dependency change and commit
  the result; never hand-edit `go.sum`.

## Single-test iteration

For a single test during iteration, run Go tests directly at package scope:

- Example: `go test ./models/registration -run TestGetEntityAcceptsV1Beta2RelationshipSchemaVersion -count=1`
- General form: `go test ./path/to/package -run TestName -count=1`

Use direct `go test ./pkg -run TestName` only for focused iteration; prefer the
Makefile targets for final verification before pushing.

## PR discipline

- Tests accompany every behavioral change; run every locally-runnable test before
  requesting review. Never defer runnable coverage to reviewers or follow-up PRs.
- After adding or modifying structured errors, `make errorutil` (and usually
  `make errorutil-analyze`) must run and its artifacts must be committed - see
  [errors.md](errors.md).
- Schema-aware changes: run `cd ../schemas && make validate-schemas && make consumer-audit`
  before pushing.
