package entity

import registryentity "github.com/meshery/meshkit/models/registry/entity"

// EntityStatus is a deprecated alias for registryentity.EntityStatus.
// Deprecated: use registryentity.EntityStatus instead.
type EntityStatus = registryentity.EntityStatus

const (
	Ignored   = registryentity.Ignored
	Enabled   = registryentity.Enabled
	Duplicate = registryentity.Duplicate
)

// Status is a deprecated alias for registryentity.Status.
// Deprecated: use registryentity.Status instead.
type Status = registryentity.Status