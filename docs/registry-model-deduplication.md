# MeshModel Registry Deduplication

When registering model definitions across multiple registrants in `RegistryManager.RegisterEntity`:

- Canonical model records are keyed by model `name` and definition `version` so distinct definition versions remain separate while duplicate registrations reuse the canonical model ID.
- A unique database index (`idx_model_dbs_name_version`) enforces uniqueness across concurrent registrations, re-reading the existing canonical model on conflict.
- Registrant associations are tracked idempotently in the `registries` table, and child component model references resolve to the canonical model ID.
