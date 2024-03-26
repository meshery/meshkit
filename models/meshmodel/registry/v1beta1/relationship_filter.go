package v1beta1

import (
	"fmt"

	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha2"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"gorm.io/gorm/clause"
)

type relationshipDefinitionWithModel struct {
	v1alpha2.RelationshipDefinitionDB
	ModelDB v1beta1.ModelDB
	// acoount for overridn fields
	// v1beta1.ModelDB.Version `json:"modelVersion"`
	CategoryDB v1beta1.CategoryDB
}

// For now, only filtering by Kind and SubType are allowed.
// In the future, we will add support to query using `selectors` (using CUE)
// TODO: Add support for Model
type RelationshipFilter struct {
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
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	rf.Kind = m["kind"].(string)
}
func (relationshipFilter *RelationshipFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	// relationshipFilter, err := utils.Cast[RelationshipFilter](f)
	// if err != nil {
	// 	return nil, 0, 0, err
	// }

	var relationshipDefinitionsWithModel []relationshipDefinitionWithModel
	finder := db.Model(&v1alpha2.RelationshipDefinitionDB{}).
		Select("relationship_definition_dbs.*, model_dbs.*").
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id"). //
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")           //
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
	if relationshipFilter.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", relationshipFilter.ModelName)
	}
	if relationshipFilter.Version != "" {
		finder = finder.Where("model_dbs.version = ?", relationshipFilter.Version)
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
		Scan(&relationshipDefinitionsWithModel).Error
	if err != nil {
		fmt.Println(err.Error()) //for debugging
	}
	defs := make([]entity.Entity, len(relationshipDefinitionsWithModel))

	// remove this when reldef and reldefdb struct is consolidated.
	for _, cm := range relationshipDefinitionsWithModel {
		// Ensure correct reg is passed, rn it is dummy for sake of testing.
		// In the first query above where we do seelection i think there changes will be requrired, an when that two def and defDB structs are consolidated, using association and preload i think we can do.
		reg := v1beta1.Host{}
		rd := cm.RelationshipDefinitionDB.GetRelationshipDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db), reg))
		defs = append(defs, &rd)
	}
	// Should have count unique relationships (by model version, model name, and relationship's kind, type, subtype, version)
	return defs, count, int(count), nil
}
