package meshmodel

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

// MeshModelRegistrantData struct defines the body of the POST request that is sent to the capability
// registry (Meshery)
//
// The body contains the
// 1. Host information
// 2. Entity type
// 3. Entity
type MeshModelRegistrantData struct {
	Host       Host                 `json:"host"`
	EntityType types.CapabilityType `json:"entityType"`
	Entity     []byte               `json:"entity"` //This will be type converted to appropriate entity on server based on passed entity type
}
type Registry struct {
	ID           uuid.UUID
	RegistrantID uuid.UUID
	Entity       uuid.UUID
	Type         types.CapabilityType
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
type Host struct {
	ID        uuid.UUID `json:"-"`
	Hostname  string
	Port      int
	ContextID string
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
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

// NewRegistryManager initializes the registry manager by creating appropriate tables.
// Any new entities that are added to the registry should be migrated here into the database
func NewRegistryManager(db *database.Handler) (*RegistryManager, error) {
	if db == nil {
		return nil, fmt.Errorf("nil database handler")
	}
	rm := RegistryManager{
		db: db,
	}
	err := rm.db.AutoMigrate(
		&Registry{},
		&Host{},
		&v1alpha1.ComponentDefinitionDB{},
		&v1alpha1.RelationshipDefinitionDB{},
		&v1alpha1.Models{},
	)
	if err != nil {
		return nil, err
	}
	return &rm, nil
}
func (rm *RegistryManager) Cleanup() {
	_ = rm.db.Migrator().DropTable(
		&Registry{},
		&Host{},
		&v1alpha1.ComponentDefinitionDB{},
		&v1alpha1.Models{},
	)
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
	case v1alpha1.RelationshipDefinition:
		relationshipID, err := v1alpha1.CreateRelationship(rm.db, entity)
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
			Entity:       relationshipID,
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
	case *v1alpha1.ComponentFilter:
		en := make([]Entity, 1)
		comps := v1alpha1.GetComponents(rm.db, *filter)
		for _, comp := range comps {
			en = append(en, comp)
		}
		return en
	case *v1alpha1.RelationshipFilter:
		en := make([]Entity, 1)
		relationships := v1alpha1.GetRelationships(rm.db, *filter)
		for _, rel := range relationships {
			en = append(en, rel)
		}
		return en
	default:
		return nil
	}
}
