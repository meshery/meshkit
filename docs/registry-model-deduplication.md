# MeshModel Registry Deduplication

When registering model definitions across multiple registrants in `RegistryManager.RegisterEntity`:

- Canonical model records are keyed by model `name` and definition `version` so distinct definition versions remain separate while duplicate registrations reuse the canonical model ID.
- Before creating the unique index (`idx_model_dbs_name_version`) on `(name, version)`, existing duplicate rows (if any) are consolidated to a single canonical model ID and child/registry references are updated, then the unique index is created with duplicate-key conflict re-reading on concurrent inserts.
- Registrant associations in `registries` are guarded inside a transaction that checks for an existing link and ignores duplicate-key errors on concurrent inserts, and child component model references resolve to the canonical model ID.
