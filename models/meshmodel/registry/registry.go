package registry

import (
	"fmt"
	"strings"
	"time"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm/clause"
	"gorm.io/gorm"
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

// Entity is referred as any type of schema managed by the registry
// ComponentDefinitions and PolicyDefinitions are examples of entities
type Entity interface {
	Type() types.CapabilityType
	GetID() uuid.UUID
}

// RegistryManager instance will expose methods for registry operations & sits between the database level operations and user facing API handlers.
type RegistryManager struct {
	db *database.Handler //This database handler will be used to perform queries inside the database
}

func RegisterModel(db *database.Handler, regID,  modelID uuid.UUID) error {
	entity := Registry{
		RegistrantID: regID,
		Entity:       modelID,
		Type:         types.ModelDefinition,
	}

	byt, err := json.Marshal(entity)
	if err != nil {
		return err
	}

	entityID := uuid.NewSHA1(uuid.UUID{}, byt)
	var reg Registry
	err = db.First(&reg, "id = ?", entityID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if err == gorm.ErrRecordNotFound {
		entity.ID = entityID
		err = db.Create(&entity).Error
		if err != nil {
			return err
		}
	}

	return nil
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
		&v1alpha1.ModelDB{},
		&v1alpha1.CategoryDB{},
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
		&v1alpha1.ModelDB{},
		&v1alpha1.CategoryDB{},
		&v1alpha1.RelationshipDefinitionDB{},
	)
}
func (rm *RegistryManager) RegisterEntity(h Host, en Entity) error {
	switch entity := en.(type) {
	case v1alpha1.ComponentDefinition:
		if entity.Schema == "" { //For components with an empty schema, exit quietly
			return nil
		}

		registrantID, err := createHost(rm.db, h)
		if err != nil {
			return err
		}

		componentID, modelID, err := v1alpha1.CreateComponent(rm.db, entity)
		if err != nil {
			return err
		}

		err = RegisterModel(rm.db, registrantID, modelID)
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

func (rm *RegistryManager) GetEntities(f types.Filter) ([]Entity, *int64, *int) {
	switch filter := f.(type) {
	case *v1alpha1.ComponentFilter:
		en := make([]Entity, 0)
		comps, count, unique := v1alpha1.GetMeshModelComponents(rm.db, *filter)
		for _, comp := range comps {
			en = append(en, comp)
		}
		return en, &count, &unique
	case *v1alpha1.RelationshipFilter:
		en := make([]Entity, 0)
		relationships, count := v1alpha1.GetMeshModelRelationship(rm.db, *filter)
		for _, rel := range relationships {
			en = append(en, rel)
		}
		return en, &count, nil
	case *v1alpha1.PolicyFilter:
		en := make([]Entity, 0)
		policies := v1alpha1.GetMeshModelPolicy(rm.db, *filter)
		for _, pol := range policies {
			en = append(en, pol)
		}
		return en, nil, nil
	default:
		return nil, nil, nil
	}
}
func (rm *RegistryManager) GetModels(db *database.Handler, f types.Filter) ([]v1alpha1.Model, int64, int) {
	var m []v1alpha1.Model
	type modelWithCategories struct {
		v1alpha1.ModelDB
		v1alpha1.CategoryDB
	}

	countUniqueModels := func(models []modelWithCategories) int {
		set := make(map[string]struct{})
		for _, model := range models {
			key := model.ModelDB.Name + "@" + model.ModelDB.Version
			if _, ok := set[key]; !ok {
				set[key] = struct{}{}
			}
		}
		return len(set)
	}

	var modelWithCategoriess []modelWithCategories
	finder := db.Model(&v1alpha1.ModelDB{}).
		Select("model_dbs.*, category_dbs.*").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id") //

	// total count before pagination
	var count int64

	if mf, ok := f.(*v1alpha1.ModelFilter); ok {
		if mf.Greedy {
			if mf.Name != "" && mf.DisplayName != "" {
				finder = finder.Where("model_dbs.name LIKE ? OR model_dbs.display_name LIKE ?", "%"+mf.Name+"%", "%"+mf.DisplayName+"%")
			} else if mf.Name != "" {
				finder = finder.Where("model_dbs.name LIKE ?", "%"+mf.Name+"%")
			} else if mf.DisplayName != "" {
				finder = finder.Where("model_dbs.display_name LIKE ?", "%"+mf.DisplayName+"%")
			}
		} else {
			if mf.Name != "" {
				finder = finder.Where("model_dbs.name = ?", mf.Name)
			}
			if mf.DisplayName != "" {
				finder = finder.Where("model_dbs.display_name = ?", mf.DisplayName)
			}
		}
		if mf.Version != "" {
			finder = finder.Where("model_dbs.version = ?", mf.Version)
		}
		if mf.Category != "" {
			finder = finder.Where("category_dbs.name = ?", mf.Category)
		}
		if mf.OrderOn != "" {
			if mf.Sort == "desc" {
				finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: mf.OrderOn}, Desc: true})
			} else {
				finder = finder.Order(mf.OrderOn)
			}
		}

		finder.Count(&count)

		if mf.Limit != 0 {
			finder = finder.Limit(mf.Limit)
		}
		if mf.Offset != 0 {
			finder = finder.Offset(mf.Offset)
		}
	}
	err := finder.
		Scan(&modelWithCategoriess).Error
	if err != nil {
		fmt.Println(modelWithCategoriess)
		fmt.Println(err.Error()) //for debugging
	}

	for _, modelDB := range modelWithCategoriess {
		m = append(m, modelDB.ModelDB.GetModel(modelDB.GetCategory(db)))
	}
	return m, count, countUniqueModels(modelWithCategoriess)
}
func (rm *RegistryManager) GetCategories(db *database.Handler, f types.Filter) ([]v1alpha1.Category, int64) {
	var catdb []v1alpha1.CategoryDB
	var cat []v1alpha1.Category
	finder := rm.db.Model(&catdb)

	// total count before pagination
	var count int64

	if mf, ok := f.(*v1alpha1.CategoryFilter); ok {
		if mf.Name != "" {
			if mf.Greedy {
				finder = finder.Where("name LIKE ?", "%"+mf.Name+"%")
			} else {
				finder = finder.Where("name = ?", mf.Name)
			}
		}
		if mf.OrderOn != "" {
			if mf.Sort == "desc" {
				finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: mf.OrderOn}, Desc: true})
			} else {
				finder = finder.Order(mf.OrderOn)
			}
		}

		finder.Count(&count)

		if mf.Limit != 0 {
			finder = finder.Limit(mf.Limit)
		}
		if mf.Offset != 0 {
			finder = finder.Offset(mf.Offset)
		}
	}

	if count == 0 {
		finder.Count(&count)
	}

	_ = finder.Find(&catdb).Error
	for _, c := range catdb {
		cat = append(cat, c.GetCategory(db))
	}

	return cat, count
}
func (rm *RegistryManager) GetRegistrant(e Entity) Host {
	eID := e.GetID()
	var reg Registry
	_ = rm.db.Where("entity = ?", eID).Find(&reg).Error
	var h Host
	_ = rm.db.Where("id = ?", reg.RegistrantID).Find(&h).Error
	return h
}

func HostnameToPascalCase(input string) string {
	parts := strings.Split(input, ".")
	caser := cases.Title(language.English)
	for i, part := range parts {
		parts[i] = caser.String(part)
	}

	pascalCaseHostname := strings.Join(parts, " ")

	return pascalCaseHostname
}
