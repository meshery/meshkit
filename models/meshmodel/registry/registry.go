package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/layer5io/meshkit/database"
	models "github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta1/model"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	Connection connection.Connection `json:"connection"`
	EntityType entity.EntityType     `json:"entityType"`
	Entity     []byte                `json:"entity"` //This will be type converted to appropriate entity on server based on passed entity type
}
type Registry struct {
	ID           uuid.UUID
	RegistrantID uuid.UUID
	Entity       uuid.UUID
	Type         entity.EntityType
	CreatedAt    time.Time
	UpdatedAt    time.Time
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
		&connection.Connection{},
		&component.ComponentDefinition{},
		&relationship.RelationshipDefinition{},
		&models.PolicyDefinition{},
		&model.ModelDefinition{},
		&category.CategoryDefinition{},
	)
	if err != nil {
		return nil, err
	}
	return &rm, nil
}
func (rm *RegistryManager) Cleanup() {
	_ = rm.db.Migrator().DropTable(
		&Registry{},
		&connection.Connection{},
		&component.ComponentDefinition{},
		&model.ModelDefinition{},
		&category.CategoryDefinition{},
		&relationship.RelationshipDefinition{},
	)
}
func (rm *RegistryManager) RegisterEntity(h connection.Connection, en entity.Entity) (bool, bool, error) {
	registrantID, err := h.Create(rm.db)
	if err != nil {
		return true, false, err
	}

	entityID, err := en.Create(rm.db, registrantID)
	if err != nil {
		return false, true, err
	}
	id, _ := uuid.NewV4()
	entry := Registry{
		ID:           id,
		RegistrantID: registrantID,
		Entity:       entityID,
		Type:         en.Type(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = rm.db.Create(&entry).Error
	if err != nil {
		return false, false, err
	}
	return false, false, nil
}

// UpdateEntityStatus updates the ignore status of an entity based on the provided parameters.
// By default during models generation ignore is set to false
func (rm *RegistryManager) UpdateEntityStatus(ID string, status string, entityType string) error {
	// Convert string UUID to google UUID
	entityID, err := uuid.FromString(ID)
	if err != nil {
		return err
	}
	switch entityType {
	case "models":
		model := model.ModelDefinition{Id: entityID}
		err := model.UpdateStatus(rm.db, entity.EntityStatus(status))
		if err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (rm *RegistryManager) GetRegistrant(e entity.Entity) connection.Connection {
	eID := e.GetID()
	var reg Registry
	_ = rm.db.Where("entity = ?", eID).Find(&reg).Error
	var h connection.Connection
	_ = rm.db.Where("id = ?", reg.RegistrantID).Find(&h).Error
	return h
}

// to be removed
func (rm *RegistryManager) GetRegistrants(f *models.HostFilter) ([]models.MeshModelHostsWithEntitySummary, int64, error) {
	var result []models.MesheryHostSummaryDB
	var totalcount int64
	db := rm.db

	query := db.Table("connections c").
		Count(&totalcount).
		Select("c.*, " +
			"COUNT(CASE WHEN r.type = 'component' THEN 1 END)  AS components, " +
			"COUNT(CASE WHEN r.type = 'model' THEN 1 END) AS models," +
			"COUNT(CASE WHEN r.type = 'relationship' THEN 1 END) AS relationships, " +
			"COUNT(CASE WHEN r.type = 'policy' THEN 1 END) AS policies").
		Joins("LEFT JOIN registries r ON c.id = r.registrant_id").
		Group("c.id")

	if f.DisplayName != "" {
		query = query.Where("kind LIKE ?", "%"+f.DisplayName+"%")
	}

	if f.OrderOn != "" {
		if f.Sort == "desc" {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: f.OrderOn}, Desc: true})
		} else {
			query = query.Order(f.OrderOn)
		}
	} else {
		query = query.Order("kind")
	}

	query = query.Offset(f.Offset)
	if f.Limit != 0 {
		query = query.Limit(f.Limit)
	}

	err := query.Scan(&result).Error

	if err != nil {
		return nil, 0, err
	}

	var response []models.MeshModelHostsWithEntitySummary

	for _, r := range result {
		res := models.MeshModelHostsWithEntitySummary{
			Connection: r.Connection,
			Summary: models.EntitySummary{
				Models:        r.Models,
				Components:    r.Components,
				Relationships: r.Relationships,
			},
		}
		response = append(response, res)
	}

	return response, totalcount, nil
}

func (rm *RegistryManager) GetEntities(f entity.Filter) ([]entity.Entity, int64, int, error) {
	return f.Get(rm.db)
}

func (rm *RegistryManager) GetEntityById(f entity.Filter) (entity.Entity, error) {
	return f.GetById(rm.db)
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
