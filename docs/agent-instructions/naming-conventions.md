# Identifier Naming Conventions

This repo adheres to the canonical camelCase-wire identifier-naming contract of the
Meshery ecosystem.

**Wire is camelCase everywhere; DB is snake_case; Go fields follow Go idiom; the ORM layer is the sole translation boundary.**

## Authority

- Authoritative source: `meshery/schemas/AGENTS.md § Casing rules at a glance`.
- Reader-friendly directory: <https://github.com/meshery/schemas/blob/master/docs/identifier-naming-contributor-guide.md>
  (the 26-row naming table with before/after and do/don't examples).
- The contract is not optional; deviations block PRs via the schemas consumer-audit CI
  gate. On any conflict, schemas wins - file discrepancies as issues against
  `meshery/schemas`, not locally.

## MeshKit-specific scope

meshkit: Go-only surface. Struct fields follow Go idiom (UserID); json tags are
camelCase; db/gorm column mappings are snake_case. MeshKit is NOT the schema source of
truth - types that mirror schema constructs come from `github.com/meshery/schemas`; do
not redeclare them locally.

MeshKit utilities that expose wire-facing identifiers (error constants, structured log
keys, event envelopes consumed by Meshery Server or Meshery Cloud) follow the
ecosystem-wide camelCase-on-the-wire contract.

## Per-layer forms relevant to MeshKit

| Layer | Form | Example |
|---|---|---|
| Go struct field | PascalCase with Go-idiomatic initialisms | `UserID`, `OrgID`, `CreatedAt` |
| `json:` tag | camelCase | `json:"userId"`, `json:"orgId"` |
| DB column / gorm mapping | snake_case | `user_id`, `org_id`, `created_at` |

## Hard rules

- `Id` (camelCase), never `ID`, in URL params, JSON tags, and TypeScript properties.
  (Go struct field names still use the Go-idiomatic `ID` initialism; only wire-facing
  identifiers use `Id`.)
- Never introduce a `json:` tag that matches the `db:`/gorm column name on a DB-backed
  field: wire is camel, DB is snake, they differ by design.
- Never mix casing conventions within a single resource.
- Schema-aware changes require running
  `cd ../schemas && make validate-schemas && make consumer-audit` before pushing.
