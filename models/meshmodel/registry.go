package meshmodel

import (
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

type Registry struct {
	ID           uuid.UUID
	RegistrantID uuid.UUID
	Entity       uuid.UUID
	Type         types.CapabilityType
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
type Host struct {
	ID        uuid.UUID
	Hostname  string
	Port      int
	ContextID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func createHost(db *database.Handler, h Host) (uuid.UUID, error) {
	h.ID = uuid.New()
	err := db.Create(&h).Error
	return h.ID, err
}

// Entity is referred as any type of schema managed by the registry
// ComponentDefinitions and PolicyDefinitions are examples of entities
type Entity interface {
	Type() types.CapabilityType
}

// RegistryManager instance will expose methods for registry operations & sits between the database level operations and user facing API handlers.
type RegistryManager struct {
	db *database.Handler //This database handler will be used to perform queries inside the database
}

func (rm *RegistryManager) RegisterEntity(h Host, en Entity) error {
	switch entity := en.(type) {
	case v1alpha1.ComponentDefinition:
		componentID, err := v1alpha1.CreateComponent(rm.db, entity)
		if err != nil {
			return err
		}
		registrantID, err := createHost(rm.db, h)
		if err != nil {
			return err
		}
		entry := Registry{
			ID:           uuid.New(),
			RegistrantID: registrantID,
			Entity:       componentID,
			Type:         en.Type(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		return rm.db.Create(&entry).Error
	//Add logic for Policies and other entities below
	default:
		return nil
	}

}

func (rm *RegistryManager) GetEntities(f types.Filter) []Entity {
	switch filter := f.(type) {
	case v1alpha1.ComponentFilter:
		en := make([]Entity, 1)
		comps := v1alpha1.GetComponents(rm.db, filter)
		for _, comp := range comps {
			en = append(en, comp)
		}
		return en
	default:
		return nil
	}

}
