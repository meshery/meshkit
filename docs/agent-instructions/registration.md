# MeshModel Registration Pipeline

Read this before touching `models/registration/` or `models/meshmodel/registry/`.

## Shape of the pipeline

- `models/registration/` ingests model packages and assembles a `PackagingUnit`: exactly
  one `model.ModelDefinition` plus any number of components, relationships, and
  connections associated with it.
- A `RegistrationHelper` (constructed with an SVG base directory, a `RegistryManager`,
  and a `RegistrationErrorStore`) drives `Register(entity)` for each
  `RegisterableEntity` - currently directory (`dir.go`), tar (`tar.go`), and OCI
  (`oci.go`) sources.
- `models/meshmodel/registry/` persists registrants, models, components, and
  relationships through GORM-backed helpers (`RegistryManager`), with versioned entity
  handling under `registry/v1alpha3` and `registry/v1beta1`.

## Rules

- **One model definition per imported package/directory.** Registration assumes an
  imported package/directory contains exactly one model definition and then any number
  of components/relationships associated with it.
- Entity types come from `github.com/meshery/schemas` (`model`, `component`,
  `relationship`, `connection` definitions). MeshKit is NOT the schema source of truth;
  never redeclare schema types locally - focus on how MeshKit loads, normalizes,
  registers, and persists those entities.

## Permissive-input behaviors (intentional, preserve them)

Registration code is intentionally permissive on input shape:

- Directory import recursively unwraps nested zip/tar/OCI content.
- YAML is normalized to JSON before entity detection and unmarshal.
- Registration accepts both legacy and canonical schema-version strings for models,
  components, and relationships where compatibility is required. Regression coverage
  exists, e.g.
  `go test ./models/registration -run TestGetEntityAcceptsV1Beta2RelationshipSchemaVersion -count=1`.

Do not tighten these behaviors without coordinating with every downstream importer
(Meshery Server model import, mesheryctl, generators).

## SVG mutation caveat

Registration mutates asset references as part of ingestion: model and component SVG
fields (color/white/complete SVGs) are written to the filesystem under the configured
SVG base directory and the in-memory fields are replaced with file paths before
persistence through `RegistryManager` (see `models/registration/svg_helper.go`). Code
that inspects a `PackagingUnit` after registration sees file paths, not inline SVG
content. Keep this in mind when adding fields that carry embedded assets.
