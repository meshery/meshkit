package meshmodel

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"gorm.io/gorm/clause"
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
	Doc(v1alpha1.DocFormat, *database.Handler) string
	GetID() uuid.UUID
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
		&v1alpha1.PolicyDefinitionDB{},
		&v1alpha1.Model{},
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
		&v1alpha1.Model{},
		&v1alpha1.RelationshipDefinitionDB{},
	)
}
func (rm *RegistryManager) RegisterEntity(h Host, en Entity) error {
	switch entity := en.(type) {
	case v1alpha1.ComponentDefinition:
		if entity.Schema == "" { //For components with an empty schema, exit quietly
			return nil
		}
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
	case v1alpha1.PolicyDefinition:
		policyID, err := v1alpha1.CreatePolicy(rm.db, entity)
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
			Entity:       policyID,
			Type:         en.Type(),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		return rm.db.Create(&entry).Error

	default:
		return nil
	}
}

func (rm *RegistryManager) GetEntities(f types.Filter) []Entity {
	switch filter := f.(type) {
	case *v1alpha1.ComponentFilter:
		en := make([]Entity, 0)
		comps := v1alpha1.GetMeshModelComponents(rm.db, *filter)
		for _, comp := range comps {
			en = append(en, comp)
		}
		return en
	case *v1alpha1.RelationshipFilter:
		en := make([]Entity, 0)
		relationships := v1alpha1.GetMeshModelRelationship(rm.db, *filter)
		for _, rel := range relationships {
			en = append(en, rel)
		}
		return en
	case *v1alpha1.PolicyFilter:
		en := make([]Entity, 0)
		policies := v1alpha1.GetMeshModelPolicy(rm.db, *filter)
		for _, pol := range policies {
			en = append(en, pol)
		}
		return en
	default:
		return nil
	}
}

func (rm *RegistryManager) GetEntityDoc(f types.Filter) string {
	switch filter := f.(type) {
	case *v1alpha1.ComponentFilter:
		comps := v1alpha1.GetMeshModelComponents(rm.db, *filter)
		if len(comps) == 0 {
			return ""
		}
		return comps[0].Doc(v1alpha1.HTMLFormat, rm.db)
	default:
		return ""
	}
}
func (rm *RegistryManager) GetModels(f types.Filter) []v1alpha1.Model {
	var mod []v1alpha1.Model
	finder := rm.db.Model(&mod)
	if mf, ok := f.(*v1alpha1.ModelFilter); ok {
		if mf.Greedy {
			if mf.Name != "" && mf.DisplayName != "" {
				finder = finder.Where("name LIKE ? OR display_name LIKE ?", mf.Name+"%", mf.DisplayName+"%")
			} else if mf.Name != "" {
				finder = finder.Where("name LIKE ?", mf.Name+"%")
			} else if mf.DisplayName != "" {
				finder = finder.Where("display_name LIKE ?", mf.DisplayName+"%")
			}
		} else {
			if mf.Name != "" {
				finder = finder.Where("name = ?", mf.Name)
			}
			if mf.DisplayName != "" {
				finder = finder.Where("display_name = ?", mf.DisplayName)
			}
		}
		if mf.Version != "" {
			finder = finder.Where("version = ?", mf.Version)
		}
		if mf.Category != "" {
			finder = finder.Where("category = ?", mf.Category)
		}
		if mf.OrderOn != "" {
			if mf.Sort == "desc" {
				finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: mf.OrderOn}, Desc: true})
			} else {
				finder = finder.Order(mf.OrderOn)
			}
		}
		if mf.Limit != 0 {
			finder = finder.Limit(mf.Limit)
		}
		if mf.Offset != 0 {
			finder = finder.Offset(mf.Offset)
		}
	}
	_ = finder.Find(&mod).Error
	return mod
}
func (rm *RegistryManager) GetRegistrant(e Entity) Host {
	eID := e.GetID()
	var reg Registry
	_ = rm.db.Where("entity = ?", eID).Find(&reg).Error
	var h Host
	_ = rm.db.Where("id = ?", reg.RegistrantID).Find(&h).Error
	return h
}
