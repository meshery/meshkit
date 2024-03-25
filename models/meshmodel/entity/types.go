package entity

import (
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
)

type EntityType string

const (
	ComponentDefinition    EntityType = "component"
	PolicyDefinition       EntityType = "policy"
	RelationshipDefinition EntityType = "relationship"
	Model                  EntityType = "model"
)

// Each entity will have it's own Filter implementation via which it exposes the nobs and dials to fetch entities
type Filter interface {
	Create(map[string]interface{})
	Get(db *database.Handler, f Filter) (entities []Entity, count int64, unique int, err error)
}

type Entity interface {
	Status
	Create(db *database.Handler) (entityID uuid.UUID, err error)
}
