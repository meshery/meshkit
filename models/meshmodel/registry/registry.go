package registry

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	models "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/meshkit/models/meshmodel/entity"
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

type EntityCacheValue struct {
	Entities []entity.Entity
	Count    int64
	Unique   int
	err      error
}

// RegistryEntityCache is a thread-safe cache for storing entity query results.
//
// This cache maps entity filters (`entity.Filter`) to their corresponding results (`EntityCacheValue`).
// It uses `sync.Map` for safe concurrent access, making it suitable for multi-goroutine environments.
//
// The caller is responsible for managing the cache's lifecycle, including eviction and expiration if needed.
type RegistryEntityCache struct {
	cache sync.Map
}

func filterKey(f entity.Filter) string {
	// Convert the filter to a unique string representation (adjust as needed)
	return fmt.Sprintf("%v", f)
}

// Get retrieves a cached value
func (c *RegistryEntityCache) Get(f entity.Filter) (EntityCacheValue, bool) {
	value, exists := c.cache.Load(filterKey(f))
	if exists {
		return value.(EntityCacheValue), true
	}
	return EntityCacheValue{}, false
}

// Set stores a value in the cache
func (c *RegistryEntityCache) Set(f entity.Filter, value EntityCacheValue) {
	c.cache.Store(filterKey(f), value)
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
	var totalConnectionsCount int64
	db := rm.db

	query := db.Table("connections c").
    Count(&totalConnectionsCount).
    Select("MIN(c.id) as id, c.kind, MIN(c.name) as name, MIN(c.type) as type, MIN(c.status) as status, " +
        "COUNT(CASE WHEN r.type = 'component' THEN 1 END) as components, " +
        "COUNT(CASE WHEN r.type = 'model' THEN 1 END) as models, " +
        "COUNT(CASE WHEN r.type = 'relationship' THEN 1 END) as relationships, " +
        "COUNT(CASE WHEN r.type = 'policy' THEN 1 END) as policies").
    Joins("LEFT JOIN registries r ON c.id = r.registrant_id").
    Group("c.kind")

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
	nonRegistantCount := int64(0)
	for _, r := range result {

		if r.Models == 0 {
			nonRegistantCount++
			continue
		}

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

	registrantCount := (totalConnectionsCount - nonRegistantCount)

	return response, registrantCount, nil
}

func (rm *RegistryManager) GetEntities(f entity.Filter) ([]entity.Entity, int64, int, error) {
	return f.Get(rm.db)
}

// GetEntitiesMemoized retrieves entities based on the provided filter `f`, using a concurrent-safe cache to optimize performance.
//
// ## Cache Behavior:
//   - **Cache Hit**: If the requested entities are found in the `cache`, the function returns the cached result immediately, avoiding a redundant query.
//   - **Cache Miss**: If the requested entities are *not* found in the cache, the function fetches them from the registry using `rm.GetEntities(f)`,
//     stores the result in the `cache`, and returns the newly retrieved entities.
//
// ## Concurrency and Thread Safety:
// - The `cache` is implemented using `sync.Map`, ensuring **safe concurrent access** across multiple goroutines.
// - `sync.Map` is optimized for scenarios where **reads significantly outnumber writes**, making it well-suited for caching use cases.
//
// ## Ownership and Responsibility:
//   - **RegistryManager (`rm`)**: Owns the logic for retrieving entities from the source when a cache miss occurs.
//   - **Caller Ownership of Cache**: The caller is responsible for providing and managing the `cache` instance.
//     This function does *not* handle cache eviction, expiration, or memory constraints—those concerns must be managed externally.
//
// ## Parameters:
// - `f entity.Filter`: The filter criteria used to retrieve entities.
// - `cache *RegistryEntityCache`: A pointer to a concurrent-safe cache (`sync.Map`) that stores previously retrieved entity results.
//
// ## Returns:
// - `[]entity.Entity`: The list of retrieved entities (either from cache or freshly fetched).
// - `int64`: The total count of entities matching the filter.
// - `int`: The number of unique entities found.
// - `error`: An error if the retrieval operation fails.
func (rm *RegistryManager) GetEntitiesMemoized(f entity.Filter, cache *RegistryEntityCache) ([]entity.Entity, int64, int, error) {

	// Attempt to retrieve from cache
	if cachedEntities, exists := cache.Get(f); exists && len(cachedEntities.Entities) > 0 {
		return cachedEntities.Entities, cachedEntities.Count, cachedEntities.Unique, cachedEntities.err
	}

	// Fetch from source if cache miss
	entities, count, unique, err := rm.GetEntities(f)

	// Store result in cache
	cache.Set(f, EntityCacheValue{
		Entities: entities,
		Count:    count,
		Unique:   unique,
		err:      err,
	})

	return entities, count, unique, err
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
