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
	core "github.com/meshery/schemas/models/core"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/category"
	connectionv1beta1 "github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/meshery/schemas/models/v1beta3/component"
	connectionv1beta3 "github.com/meshery/schemas/models/v1beta3/connection"
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
	Connection connectionv1beta3.Connection `json:"connection"`
	EntityType entity.EntityType            `json:"entityType"`
	Entity     []byte                       `json:"entity"` //This will be type converted to appropriate entity on server based on passed entity type
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
	ID           core.Uuid
	RegistrantID core.Uuid
	Entity       core.Uuid
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
		&connectionv1beta1.Connection{},
		&component.ComponentDefinition{},
		&relationship.RelationshipDefinition{},
		&connectionv1beta3.ConnectionDefinition{},
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
		&connectionv1beta1.Connection{},
		&component.ComponentDefinition{},
		&connectionv1beta3.ConnectionDefinition{},
		&model.ModelDefinition{},
		&category.CategoryDefinition{},
		&relationship.RelationshipDefinition{},
	)
}
func (rm *RegistryManager) RegisterEntity(h connectionv1beta3.Connection, en entity.Entity) (bool, bool, error) {
	// The registrant/host is exposed as a v1beta3 connection, but it is a live
	// connection persisted in the `connections` table (the table joined by
	// GetRegistrants and the component filter). The v1beta3 ConnectionDefinition
	// helpers target `connection_definition_dbs`, require a parent host id, and
	// generate a random id - none of which fit a registrant. We therefore adapt
	// the host to the v1beta1 connection and reuse its idempotent, content-addressed
	// Create so repeated registrations of the same host reuse a single registrant row.
	registrant := RegistrantHostToV1beta1(h)
	registrantID, err := registrant.Create(rm.db)
	if err != nil {
		return true, false, err
	}

	entityID, err := en.Create(rm.db, registrantID)
	if err != nil {
		return false, true, err
	}
	id, err := uuid.NewV4()
	if err != nil {
		return false, false, err
	}
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
		model := model.ModelDefinition{ID: entityID}
		err := model.UpdateStatus(rm.db, entity.EntityStatus(status))
		if err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (rm *RegistryManager) GetRegistrant(e entity.Entity) connectionv1beta3.Connection {
	eID := e.GetID()
	var reg Registry
	_ = rm.db.Where("entity = ?", eID).Find(&reg).Error
	// Registrants live in the `connections` table as v1beta1 connections; read them
	// there and adapt to the v1beta3 connection type now exposed by the registry API.
	var h connectionv1beta1.Connection
	_ = rm.db.Where("id = ?", reg.RegistrantID).Find(&h).Error
	return RegistrantHostToV1beta3(h)
}

// RegistrantHostToV1beta3 adapts the schema-fixed v1beta1 registrant/host
// connection (e.g. model.ModelDefinition.Registrant) to the v1beta3 connection
// type now exposed by the registry API. Only the fields backed by the
// `connections` table are carried over. Consumers (e.g. Meshery) that hold a
// v1beta1 registrant should use this helper to build the v1beta3 host passed to
// RegisterEntity.
func RegistrantHostToV1beta3(c connectionv1beta1.Connection) connectionv1beta3.Connection {
	return connectionv1beta3.Connection{
		ID:             c.ID,
		Name:           c.Name,
		CredentialID:   c.CredentialID,
		ConnectionType: c.Type,
		SubType:        c.SubType,
		Kind:           c.Kind,
		Metadata:       c.Metadata,
		Status:         connectionv1beta3.ConnectionStatus(c.Status),
		Owner:          c.UserID,
		SchemaVersion:  c.SchemaVersion,
	}
}

// RegistrantHostToV1beta1 is the inverse of RegistrantHostToV1beta3. It maps a
// v1beta3 registrant/host back to the v1beta1 connection persisted in the
// `connections` table so the existing idempotent Create path can be reused.
func RegistrantHostToV1beta1(h connectionv1beta3.Connection) connectionv1beta1.Connection {
	return connectionv1beta1.Connection{
		ID:            h.ID,
		Name:          h.Name,
		CredentialID:  h.CredentialID,
		Type:          h.ConnectionType,
		SubType:       h.SubType,
		Kind:          h.Kind,
		Metadata:      h.Metadata,
		Status:        connectionv1beta1.ConnectionStatus(h.Status),
		UserID:        h.Owner,
		SchemaVersion: h.SchemaVersion,
	}
}

// to be removed
func (rm *RegistryManager) GetRegistrants(f *models.HostFilter) ([]models.MeshModelHostsWithEntitySummary, int64, error) {
	var result []models.MesheryHostSummaryDB
	var totalConnectionsCount int64
	db := rm.db

	query := db.Table("connections c").
		Count(&totalConnectionsCount).
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
