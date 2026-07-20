package v1beta1

import (
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/registry/entity"
	"github.com/meshery/meshkit/models/registry/manager"
	connectionv1beta3 "github.com/meshery/schemas/models/v1beta3/connection"
	"gorm.io/gorm/clause"
)

// ConnectionFilter exposes the knobs to fetch connection definitions
// (entity.ConnectionDefinition) registered in the registry.
type ConnectionFilter struct {
	Id        string
	Name      string
	Kind      string
	Type      string
	ModelName string
	Version   string
	Status    string
	Greedy    bool   //when set to true - instead of an exact match, name will be matched as a substring
	Sort      string //asc or desc. Default behavior is asc
	OrderOn   string
	Limit     int //If 0 or unspecified then all records are returned and limit is not used
	Offset    int
}

func (cf *ConnectionFilter) GetById(db *database.Handler) (entity.Entity, error) {
	c := &connectionv1beta3.ConnectionDefinition{}
	// model_reference is a JSON column on connection_definition_dbs (hydrated by
	// the type's Scan), not a GORM association, so there is nothing to Preload.
	err := db.First(c, "id = ?", cf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, cf.Id)
	}
	return c, err
}

// Create the filter from map[string]interface{}
func (cf *ConnectionFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	if name, ok := m["name"].(string); ok {
		cf.Name = name
	}
}

func (cf *ConnectionFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	var connectionDefinitions []connectionv1beta3.ConnectionDefinition

	// Query the connection_definition_dbs table directly. The owning model is no
	// longer a GORM association/FK (model_id is gone); it is carried inline as the
	// JSON `model_reference` column, which the type's Scan hydrates on Find, so
	// there is nothing to Preload. We also avoid a JOIN with a `table.*` SELECT:
	// GORM's SQLite dialector mis-quotes `connection_definition_dbs.*` as a single
	// identifier (notably inside the Count() rewrite), yielding
	// "no such column: connection_definition_dbs.*". Filtering by model is done via
	// JSON extraction on model_reference so the query stays single-table.
	finder := db.Model(&connectionv1beta3.ConnectionDefinition{})

	if cf.ModelName != "" && cf.ModelName != "all" {
		finder = finder.Where("connection_definition_dbs.model_reference->>'name' = ?", cf.ModelName)
		if cf.Version != "" {
			finder = finder.Where("connection_definition_dbs.model_reference->'model'->>'version' = ?", cf.Version)
		}
	}

	if cf.Name != "" {
		if cf.Greedy {
			finder = finder.Where("connection_definition_dbs.name LIKE ?", "%"+cf.Name+"%")
		} else {
			finder = finder.Where("connection_definition_dbs.name = ?", cf.Name)
		}
	}
	if cf.Kind != "" {
		finder = finder.Where("connection_definition_dbs.kind = ?", cf.Kind)
	}
	if cf.Type != "" {
		// The Go field is ConnectionType to avoid colliding with the registry
		// Entity interface's Type(); the DB column is `connection_type`.
		finder = finder.Where("connection_definition_dbs.connection_type = ?", cf.Type)
	}
	if cf.Status != "" {
		finder = finder.Where("connection_definition_dbs.status = ?", cf.Status)
	}
	if cf.Id != "" {
		finder = finder.Where("connection_definition_dbs.id = ?", cf.Id)
	}

	if cf.OrderOn != "" {
		if cf.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: cf.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(cf.OrderOn)
		}
	} else {
		finder = finder.Order("connection_definition_dbs.name")
	}

	var count int64
	finder.Count(&count)

	finder = finder.Offset(cf.Offset)
	if cf.Limit != 0 {
		finder = finder.Limit(cf.Limit)
	}

	if err := finder.Find(&connectionDefinitions).Error; err != nil {
		return nil, 0, 0, err
	}

	defs := make([]entity.Entity, 0, len(connectionDefinitions))
	for i := range connectionDefinitions {
		defs = append(defs, &connectionDefinitions[i])
	}

	return defs, count, len(defs), nil
}
