package registry

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	types "github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
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
	Host       Host             `json:"host"`
	EntityType types.EntityType `json:"entityType"`
	Entity     []byte           `json:"entity"` //This will be type converted to appropriate entity on server based on passed entity type
}
type Registry struct {
	ID           uuid.UUID
	RegistrantID uuid.UUID
	Entity       uuid.UUID
	Type         types.EntityType
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Entity is referred as any type of schema managed by the registry
// ComponentDefinitions and PolicyDefinitions are examples of entities
type Entity interface {
	Type() types.EntityType
	GetID() uuid.UUID
}

// RegistryManager instance will expose methods for registry operations & sits between the database level operations and user facing API handlers.
type RegistryManager struct {
	db *database.Handler //This database handler will be used to perform queries inside the database
}

// Registers models into registries table.
func registerModel(db *database.Handler, regID, modelID uuid.UUID) error {
	entity := Registry{
		RegistrantID: regID,
		Entity:       modelID,
		Type:         types.Model,
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
		isAnnotation, _ := entity.Metadata["isAnnotation"].(bool)
		if entity.Schema == "" && !isAnnotation { //For components which an empty schema and is not an annotation, exit quietly
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

		err = registerModel(rm.db, registrantID, modelID)
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

		registrantID, err := createHost(rm.db, h)
		if err != nil {
			return err
		}

		relationshipID, modelID, err := v1alpha1.CreateRelationship(rm.db, entity)
		if err != nil {
			return err
		}

		err = registerModel(rm.db, registrantID, modelID)
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
		registrantID, err := createHost(rm.db, h)
		if err != nil {
			return err
		}

		policyID, modelID, err := v1alpha1.CreatePolicy(rm.db, entity)
		if err != nil {
			return err
		}

		err = registerModel(rm.db, registrantID, modelID)
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

// UpdateEntityIgnoreStatus updates the ignore status of an entity based on the provided parameters.
// By default during models generation ignore is set to false
func (rm *RegistryManager) UpdateEntityStatus(ID string, status string, entity string) error {
	// Convert string UUID to google UUID
	entityID, err := uuid.Parse(ID)
	if err != nil {
		return err
	}
	switch entity {
	case "models":
		err := v1alpha1.UpdateModelsStatus(rm.db, entityID, status)
		if err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (rm *RegistryManager) GetRegistrants(f *v1alpha1.HostFilter) ([]v1alpha1.MeshModelHostsWithEntitySummary, int64, error) {
	var result []v1alpha1.MesheryHostSummaryDB
	var totalcount int64
	db := rm.db

	query := db.Table("hosts h").
		Count(&totalcount).
		Select("h.id AS host_id, h.hostname, h.port, " +
			"COUNT(CASE WHEN r.type = 'component' THEN 1 END)  AS components, " +
			"COUNT(CASE WHEN r.type = 'model' THEN 1 END) AS models," +
			"COUNT(CASE WHEN r.type = 'relationship' THEN 1 END) AS relationships, " +
			"COUNT(CASE WHEN r.type = 'policy' THEN 1 END) AS policies").
		Joins("LEFT JOIN registries r ON h.id = r.registrant_id").
		Group("h.id, h.hostname, h.port")

	if f.DisplayName != "" {
		query = query.Where("hostname LIKE ?", "%"+f.DisplayName+"%")
	}

	if f.OrderOn != "" {
		if f.Sort == "desc" {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: f.OrderOn}, Desc: true})
		} else {
			query = query.Order(f.OrderOn)
		}
	} else {
		query = query.Order("hostname")
	}

	query = query.Offset(f.Offset)
	if f.Limit != 0 {
		query = query.Limit(f.Limit)
	}

	err := query.Scan(&result).Error

	if err != nil {
		return nil, 0, err
	}

	var response []v1alpha1.MeshModelHostsWithEntitySummary

	for _, r := range result {
		res := v1alpha1.MeshModelHostsWithEntitySummary{
			ID:       r.HostID,
			Hostname: HostnameToPascalCase(r.Hostname),
			Port:     r.Port,
			Summary: v1alpha1.EntitySummary{
				Models:        r.Models,
				Components:    r.Components,
				Relationships: r.Relationships,
			},
		}
		response = append(response, res)
	}

	return response, totalcount, nil
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
		Registry
		Host
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
		Select("model_dbs.*, category_dbs.*", "registries.*", "hosts.*").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN registries ON registries.entity = model_dbs.id").
		Joins("JOIN hosts ON hosts.id = registries.registrant_id")

	// total count before pagination
	var count int64

	// include components and relationships in response body
	var includeComponents, includeRelationships bool

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
		if mf.Annotations == "true" {
			finder = finder.Where("model_dbs.metadata->>'isAnnotation' = true")
		} else if mf.Annotations == "false" {
			finder = finder.Where("model_dbs.metadata->>'isAnnotation' = false")
		}
		if mf.Version != "" {
			finder = finder.Where("model_dbs.version = ?", mf.Version)
		}
		if mf.Category != "" {
			finder = finder.Where("category_dbs.name = ?", mf.Category)
		}
		if mf.Registrant != "" {
			finder = finder.Where("hosts.hostname = ?", mf.Registrant)
		}
		if mf.Annotations == "true" {
			finder = finder.Where("model_dbs.metadata->>'isAnnotation' = true")
		} else if mf.Annotations == "false" {
			finder = finder.Where("model_dbs.metadata->>'isAnnotation' = false")
		}
		if mf.OrderOn != "" {
			if mf.Sort == "desc" {
				finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: mf.OrderOn}, Desc: true})
			} else {
				finder = finder.Order(mf.OrderOn)
			}
		} else {
			finder = finder.Order("display_name")
		}

		finder.Count(&count)

		if mf.Limit != 0 {
			finder = finder.Limit(mf.Limit)
		}
		if mf.Offset != 0 {
			finder = finder.Offset(mf.Offset)
		}
		if mf.Status != "" {
			finder = finder.Where("model_dbs.status = ?", mf.Status)
		}
		includeComponents = mf.Components
		includeRelationships = mf.Relationships
	}
	err := finder.
		Scan(&modelWithCategoriess).Error
	if err != nil {
		fmt.Println(modelWithCategoriess)
		fmt.Println(err.Error()) //for debugging
	}

	for _, modelDB := range modelWithCategoriess {
		model := modelDB.ModelDB.GetModel(modelDB.GetCategory(db))
		host := rm.GetRegistrant(model)
		model.HostID = host.ID
		model.HostName = host.Hostname
		model.DisplayHostName = host.Hostname

		if includeComponents {
			var components []v1alpha1.ComponentDefinitionDB
			finder := db.Model(&v1alpha1.ComponentDefinitionDB{}).
				Select("component_definition_dbs.id, component_definition_dbs.kind,component_definition_dbs.display_name, component_definition_dbs.api_version, component_definition_dbs.metadata").
				Where("component_definition_dbs.model_id = ?", model.ID)
			if err := finder.Scan(&components).Error; err != nil {
				fmt.Println(err)
			}
			model.Components = components
		}
		if includeRelationships {
			var relationships []v1alpha1.RelationshipDefinitionDB
			finder := db.Model(&v1alpha1.RelationshipDefinitionDB{}).
				Select("relationship_definition_dbs.*").
				Where("relationship_definition_dbs.model_id = ?", model.ID)
			if err := finder.Scan(&relationships).Error; err != nil {
				fmt.Println(err)
			}
			model.Relationships = relationships
		}

		m = append(m, model)
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
