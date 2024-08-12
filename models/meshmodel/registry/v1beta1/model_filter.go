package v1beta1

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
	v1beta1 "github.com/meshery/schemas/models/v1beta1/model"

	"gorm.io/gorm/clause"
)

type ModelFilter struct {
	Id          string
	Name        string
	Registrant  string //name of the registrant for a given model
	DisplayName string //If Name is already passed, avoid passing Display name unless greedy=true, else the filter will translate to an AND returning only the models where name and display name match exactly. Ignore, if this behavior is expected.
	Greedy      bool   //when set to true - instead of an exact match, name will be prefix matched. Also an OR will be performed of name and display_name
	Version     string
	Category    string
	OrderOn     string
	Sort        string //asc or desc. Default behavior is asc
	Limit       int    //If 0 or unspecified then all records are returned and limit is not used
	Offset      int
	Annotations string //When this query parameter is "true", only models with the "isAnnotation" property set to true are returned. When  this query parameter is "false", all models except those considered to be annotation models are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.

	// When these are set to true, we also retrieve components/relationships associated with the model.
	Components    bool
	Relationships bool
	Status        string
}

// Create the filter from map[string]interface{}
func (mf *ModelFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	mf.Name = m["name"].(string)
}

func countUniqueModels(models []v1beta1.ModelDefinition) int {
	set := make(map[string]struct{})
	for _, model := range models {
		key := model.Name + "@" + model.Model.Version
		if _, ok := set[key]; !ok {
			set[key] = struct{}{}
		}
	}
	return len(set)
}

func (mf *ModelFilter) GetById(db *database.Handler) (entity.Entity, error) {
	m := &v1beta1.Model{}

	// Retrieve the model by ID
	err := db.First(m, "id = ?", mf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, mf.Id)
	}

	// Include components if requested
	if mf.Components {
		var components []v1beta1.ComponentDefinition
		componentFinder := db.Model(&v1beta1.ComponentDefinition{}).
			Select("component_definition_dbs.id, component_definition_dbs.component, component_definition_dbs.display_name, component_definition_dbs.metadata, component_definition_dbs.schema_version, component_definition_dbs.version").
			Where("component_definition_dbs.model_id = ?", m.ID)
		if err := componentFinder.Scan(&components).Error; err != nil {
			return nil, err
		}
		m.Components = components
	}

	// Include relationships if requested
	if mf.Relationships {
		var relationships []v1alpha2.RelationshipDefinition
		relationshipFinder := db.Model(&v1alpha2.RelationshipDefinition{}).
			Select("relationship_definition_dbs.*").
			Where("relationship_definition_dbs.model_id = ?", m.ID)
		if err := relationshipFinder.Scan(&relationships).Error; err != nil {
			return nil, err
		}
		m.Relationships = relationships
	}

	return m, nil
}

func (mf *ModelFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {

	var modelWithCategories []model.ModelDefinition

	finder := db.Model(&model.ModelDefinition{}).Preload("Category").Preload("Registrant").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN registries ON registries.entity = model_dbs.id").
		Joins("JOIN connections ON connections.id = registries.registrant_id")

	// total count before pagination
	var count int64

	// include components and relationships in response body
	var includeComponents, includeRelationships bool

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
		finder = finder.Where("model_dbs.model->>'version' = ?", mf.Version)
	}
	if mf.Category != "" {
		finder = finder.Where("category_dbs.name = ?", mf.Category)
	}
	if mf.Registrant != "" {
		finder = finder.Where("connections.kind = ?", mf.Registrant)
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

	status := "enabled"

	if mf.Status != "" {
		status = mf.Status
	}

	finder = finder.Where("model_dbs.status = ?", status)

	includeComponents = mf.Components
	includeRelationships = mf.Relationships

	err := finder.
		Find(&modelWithCategories).Error
	if err != nil {
		return nil, 0, 0, err
	}

	defs := []entity.Entity{}

	for _, modelDB := range modelWithCategories {
		// resolve for loop scope
		_modelDB := modelDB
		if includeComponents {
			var components []v1beta1.ComponentDefinition
			finder := db.Model(&v1beta1.ComponentDefinition{}).
				Select("component_definition_dbs.id, component_definition_dbs.component, component_definition_dbs.display_name, component_definition_dbs.metadata, component_definition_dbs.schema_version, component_definition_dbs.version").
				Where("component_definition_dbs.model_id = ?", _modelDB.ID)
			if err := finder.Scan(&components).Error; err != nil {
				return nil, 0, 0, err
			}
			_modelDB.Components = components
		}
		if includeRelationships {
			var relationships []relationship.RelationshipDefinition
			finder := db.Model(&relationship.RelationshipDefinition{}).
				Select("relationship_definition_dbs.*").
				Where("relationship_definition_dbs.model_id = ?", _modelDB.Id)
			if err := finder.Scan(&relationships).Error; err != nil {
				return nil, 0, 0, err
			}
			_modelDB.Relationships = relationships
		}
		defs = append(defs, &_modelDB)
	}
	return defs, count, countUniqueModels(modelWithCategories), nil
}
