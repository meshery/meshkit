package entity

import (
	"github.com/meshery/meshkit/database"
	core "github.com/meshery/schemas/models/core"
)

type EntityType string

const (
	ComponentDefinition    EntityType = "component"
	PolicyDefinition       EntityType = "policy"
	RelationshipDefinition EntityType = "relationship"
	Model                  EntityType = "model"
	Category               EntityType = "category"
)

// Each entity will have it's own Filter implementation via which it exposes the nobs and dials to fetch entities
type Filter interface {
	Create(map[string]interface{})
	Get(db *database.Handler) (entities []Entity, count int64, unique int, err error)
	GetById(db *database.Handler) (entity Entity, err error)
}

type Entity interface {
	// Entity is referred as any type of schema managed by the registry
	// ComponentDefinitions and PolicyDefinitions are examples of entities
	Type() EntityType
	GetEntityDetail() string
	GenerateID() (core.Uuid, error)
	GetID() core.Uuid
	Create(db *database.Handler, hostID core.Uuid) (entityID core.Uuid, err error)
}
