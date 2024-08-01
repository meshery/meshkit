package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha2"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
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
	Host       v1beta1.Host      `json:"host"`
	EntityType entity.EntityType `json:"entityType"`
	Entity     []byte            `json:"entity"` //This will be type converted to appropriate entity on server based on passed entity type
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
		&v1beta1.Host{},
		&v1beta1.ComponentDefinition{},
		&v1alpha2.RelationshipDefinition{},
		&v1beta1.PolicyDefinition{},
		&v1beta1.Model{},
		&v1beta1.Category{},
	)
	if err != nil {
		return nil, err
	}
	return &rm, nil
}
func (rm *RegistryManager) Cleanup() {
	_ = rm.db.Migrator().DropTable(
		&Registry{},
		&v1beta1.Host{},
		&v1beta1.ComponentDefinition{},
		&v1beta1.Model{},
		&v1beta1.Category{},
		&v1alpha2.RelationshipDefinition{},
	)
}
func (rm *RegistryManager) RegisterEntity(h v1beta1.Host, en entity.Entity) (bool, bool, error) {
	registrantID, err := h.Create(rm.db)
	if err != nil {
		return true, false, err
	}

	entityID, err := en.Create(rm.db, registrantID)
	if err != nil {
		return false, true, err
	}
	entry := Registry{
		ID:           uuid.New(),
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
	entityID, err := uuid.Parse(ID)
	if err != nil {
		return err
	}
	switch entityType {
	case "models":
		model := v1beta1.Model{ID: entityID}
		err := model.UpdateStatus(rm.db, entity.EntityStatus(status))
		if err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (rm *RegistryManager) GetRegistrant(e entity.Entity) v1beta1.Host {
	eID := e.GetID()
	var reg Registry
	_ = rm.db.Where("entity = ?", eID).Find(&reg).Error
	var h v1beta1.Host
	_ = rm.db.Where("id = ?", reg.RegistrantID).Find(&h).Error
	return h
}

// to be removed
func (rm *RegistryManager) GetRegistrants(f *v1beta1.HostFilter) ([]v1beta1.MeshModelHostsWithEntitySummary, int64, error) {
	var result []v1beta1.MesheryHostSummaryDB
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

	var response []v1beta1.MeshModelHostsWithEntitySummary

	for _, r := range result {
		res := v1beta1.MeshModelHostsWithEntitySummary{
			ID:       r.HostID,
			Hostname: HostnameToPascalCase(r.Hostname),
			Port:     r.Port,
			Summary: v1beta1.EntitySummary{
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

func (rm *RegistryManager) GetEntityById (f entity.Filter) (entity.Entity, error) {
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
