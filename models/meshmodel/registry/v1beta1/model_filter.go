package v1beta1

import (
	"github.com/gofrs/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"

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
	// When Trim is true it will only send necessary models data
	// like: component count, relationship count, id and name of model
	Trim bool
}

// Create the filter from map[string]interface{}
func (mf *ModelFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	mf.Name = m["name"].(string)
}

func countUniqueModels(models []model.ModelDefinition) int {
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
	m := &model.ModelDefinition{}

	// Retrieve the model by ID
	err := db.First(m, "id = ?", mf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, mf.Id)
	}

	// Include components if requested
	if mf.Components {
		var components []component.ComponentDefinition
		componentFinder := db.Model(&component.ComponentDefinition{}).
			Select("component_definition_dbs.id, component_definition_dbs.component, component_definition_dbs.display_name, component_definition_dbs.metadata, component_definition_dbs.schema_version, component_definition_dbs.version").
			Where("component_definition_dbs.model_id = ?", m.Id)
		if err := componentFinder.Scan(&components).Error; err != nil {
			return nil, err
		}
		m.Components = components
	}

	// Include relationships if requested
	if mf.Relationships {
		var relationships []relationship.RelationshipDefinition
		relationshipFinder := db.Model(&relationship.RelationshipDefinition{}).
			Select("relationship_definition_dbs.*").
			Where("relationship_definition_dbs.model_id = ?", m.Id)
		if err := relationshipFinder.Scan(&relationships).Error; err != nil {
			return nil, err
		}
		m.Relationships = relationships
	}

	return m, nil
}

func (mf *ModelFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	var modelWithCategories []model.ModelDefinition

	finder := db.Model(&model.ModelDefinition{}).
		Preload("Category").
		Preload("Registrant").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN registries ON registries.entity = model_dbs.id").
		Joins("JOIN connections ON connections.id = registries.registrant_id")

	// total count before pagination
	var count int64

	if mf.Greedy {
		if mf.Id != "" {
			finder = finder.Where("model_dbs.id = ?", mf.Id)
		}
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
	if mf.Id != "" {
		finder = finder.Where("model_dbs.id = ?", mf.Id)
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

	status := "enabled"

	if mf.Status != "" {
		status = mf.Status
	}

	finder = finder.Where("model_dbs.status = ?", status)

	finder.Count(&count)

	if mf.Limit != 0 {
		finder = finder.Limit(mf.Limit)
	}
	if mf.Offset != 0 {
		finder = finder.Offset(mf.Offset)
	}

	err := finder.Find(&modelWithCategories).Error
	if err != nil {
		return nil, 0, 0, err
	}

	if mf.Trim {
		defs := make([]entity.Entity, len(modelWithCategories))
		for i, modelDB := range modelWithCategories {
			defs[i] = &model.ModelDefinition{
				Id:          modelDB.Id,
				Name:        modelDB.Name,
				DisplayName: modelDB.DisplayName,
				Metadata:    modelDB.Metadata,
			}
		}
		return defs, count, countUniqueModels(modelWithCategories), nil
	}

	modelMap := make(map[uuid.UUID]*model.ModelDefinition, len(modelWithCategories))
	modelIds := make([]uuid.UUID, len(modelWithCategories))

	for i := range modelWithCategories {
		modelIds[i] = modelWithCategories[i].Id
		modelMap[modelWithCategories[i].Id] = &modelWithCategories[i]

		// Initialize empty slices for components and relationships
		if mf.Components {
			modelWithCategories[i].Components = make([]component.ComponentDefinition, 0)
		}
		if mf.Relationships {
			modelWithCategories[i].Relationships = make([]relationship.RelationshipDefinition, 0)
		}
	}

	if mf.Components {
		var components []component.ComponentDefinition
		if err := db.Model(&component.ComponentDefinition{}).
			Where("model_id IN ?", modelIds).
			Find(&components).Error; err != nil {
			return nil, 0, 0, err
		}

		// Map components to their models
		for _, comp := range components {
			if model, exists := modelMap[comp.ModelId]; exists {
				model.Components = append(
					model.Components.([]component.ComponentDefinition),
					comp,
				)
				model.ComponentsCount++
			}
		}
	}

	if mf.Relationships {
		var relationships []relationship.RelationshipDefinition
		if err := db.Model(&relationship.RelationshipDefinition{}).
			Where("model_id IN ?", modelIds).
			Find(&relationships).Error; err != nil {
			return nil, 0, 0, err
		}

		// Map relationships to their models
		for _, rel := range relationships {
			if model, exists := modelMap[rel.ModelId]; exists {
				model.Relationships = append(
					model.Relationships.([]relationship.RelationshipDefinition),
					rel,
				)
				model.RelationshipsCount++
			}
		}
	}

	defs := make([]entity.Entity, len(modelWithCategories))
	for i := range modelWithCategories {
		defs[i] = &modelWithCategories[i]
	}

	return defs, count, countUniqueModels(modelWithCategories), nil
}
