package v1alpha3

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/meshery/schemas/models/v1alpha3/relationship"

	"gorm.io/gorm/clause"
)

// For now, only filtering by Kind and SubType are allowed.
// In the future, we will add support to query using `selectors` (using CUE)
// TODO: Add support for Model
type RelationshipFilter struct {
	Id               string
	Kind             string
	Greedy           bool //when set to true - instead of an exact match, kind will be prefix matched
	SubType          string
	RelationshipType string
	Version          string
	ModelName        string
	OrderOn          string
	Sort             string //asc or desc. Default behavior is asc
	Limit            int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset           int
	Status           string
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	rf.Kind = m["kind"].(string)
}

func (rf *RelationshipFilter) GetById(db *database.Handler) (entity.Entity, error) {
	r := &relationship.RelationshipDefinition{}
	err := db.First(r, "id = ?", rf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, rf.Id)
	}
	return r, err
}

func (relationshipFilter *RelationshipFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {

	var relationshipDefinitionsWithModel []relationship.RelationshipDefinition
	finder := db.Model(&relationship.RelationshipDefinition{}).Preload("Model").Preload("Model.Category").
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")

	status := "enabled"

	if relationshipFilter.Status != "" {
		status = relationshipFilter.Status
	}

	finder = finder.Where("model_dbs.status = ?", status)

	if relationshipFilter.Kind != "" {
		if relationshipFilter.Greedy {
			finder = finder.Where("relationship_definition_dbs.kind LIKE ?", "%"+relationshipFilter.Kind+"%")
		} else {
			finder = finder.Where("relationship_definition_dbs.kind = ?", relationshipFilter.Kind)
		}
	}

	if relationshipFilter.RelationshipType != "" {
		finder = finder.Where("relationship_definition_dbs.type = ?", relationshipFilter.RelationshipType)
	}

	if relationshipFilter.SubType != "" {
		finder = finder.Where("relationship_definition_dbs.sub_type = ?", relationshipFilter.SubType)
	}
	if relationshipFilter.Id != "" {
		finder = finder.Where("relationship_definition_dbs.id = ?", relationshipFilter.Id)
	}
	if relationshipFilter.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", relationshipFilter.ModelName)
	}
	if relationshipFilter.Version != "" {
		finder = finder.Where("model_dbs.model->>'version' = ?", relationshipFilter.Version)
	}
	if relationshipFilter.OrderOn != "" {
		if relationshipFilter.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: relationshipFilter.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(relationshipFilter.OrderOn)
		}
	}

	var count int64
	finder.Count(&count)

	finder = finder.Offset(relationshipFilter.Offset)
	if relationshipFilter.Limit != 0 {
		finder = finder.Limit(relationshipFilter.Limit)
	}
	err := finder.
		Find(&relationshipDefinitionsWithModel).Error
	if err != nil {
		return nil, 0, 0, err
	}
	defs := make([]entity.Entity, 0, len(relationshipDefinitionsWithModel))

	for _, rd := range relationshipDefinitionsWithModel {
		// resolve for loop scope
		_rd := rd

		defs = append(defs, &_rd)
	}
	// Should have count unique relationships (by model version, model name, and relationship's kind, type, subtype, version)
	return defs, count, int(count), nil
}
