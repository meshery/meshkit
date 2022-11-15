package meshmodel

import (
	"fmt"
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
func deleteHost(db *database.Handler, h Host) error {
	return db.Where("id = ?", h.ID).Delete(&h).Error
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
		&v1alpha1.ComponentMetadataDB{},
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
		&v1alpha1.ComponentMetadataDB{},
	)
}

// Any host that unregisters will have its entry removed from the registry and all the components registered by it will be marked as INVALID status
// First mark the components invalid, then delete the host entry and finally remove the registry entry.
func (rm *RegistryManager) Unregister(h Host) error {
	entries := []Registry{}
	err := rm.db.Where("registrantid = ?", h.ID).Find(&entries).Error
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Type {
		case types.ComponentDefinition:
			err := rm.db.Model(&v1alpha1.ComponentDefinitionDB{}).Where("id = ?", entry.Entity).Update("status", v1alpha1.INVALID).Error
			if err != nil {
				return err
			}
			err = rm.db.Model(&v1alpha1.ComponentMetadataDB{}).Where("componentid = ?", entry.Entity).Update("status", v1alpha1.INVALID).Error
			if err != nil {
				return err
			}
			err = deleteHost(rm.db, h)
			if err != nil {
				return err
			}
		//Add logic for Policies and other entities below
		default:
			return nil
		}
	}
	return rm.db.Where("registrantid = ?", h.ID).Delete(&Registry{}).Error
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
	case *v1alpha1.ComponentFilter:
		en := make([]Entity, 1)
		comps := v1alpha1.GetComponents(rm.db, *filter)
		for _, comp := range comps {
			en = append(en, comp)
		}
		return en
	default:
		return nil
	}
}
