// Package entity is a deprecated compatibility shim for the registry entity
// package.
//
// It exists solely because github.com/meshery/schemas@v1.3.26 (our currently
// pinned dependency) has helper files that import this path directly by
// convention. All types here are pure aliases (type X = Y) of their
// counterparts in github.com/meshery/meshkit/models/registry/entity, so
// nothing drifts between the two paths.
//
// Deprecated: import github.com/meshery/meshkit/models/registry/entity
// directly. This shim should be deleted once meshkit's go.mod pin on
// github.com/meshery/schemas is bumped past v1.3.26 to a version that no
// longer references the old meshmodel path.
package entity

import (
	registryentity "github.com/meshery/meshkit/models/registry/entity"
)

// EntityType is a deprecated alias for registryentity.EntityType.
// Deprecated: use registryentity.EntityType instead.
type EntityType = registryentity.EntityType

const (
	ComponentDefinition    = registryentity.ComponentDefinition
	PolicyDefinition       = registryentity.PolicyDefinition
	RelationshipDefinition = registryentity.RelationshipDefinition
	Model                  = registryentity.Model
	Category               = registryentity.Category
	ConnectionDefinition   = registryentity.ConnectionDefinition
)

// Filter is a deprecated alias for registryentity.Filter.
// Deprecated: use registryentity.Filter instead.
type Filter = registryentity.Filter

// Entity is a deprecated alias for registryentity.Entity.
// Deprecated: use registryentity.Entity instead.
type Entity = registryentity.Entity
