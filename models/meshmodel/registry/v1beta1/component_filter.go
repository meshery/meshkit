package v1beta1

import (
	"fmt"

	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
)

type ComponentFilter struct {
	Name         string
	APIVersion   string
	Greedy       bool //when set to true - instead of an exact match, name will be matched as a substring
	Trim         bool //when set to true - the schema is not returned
	DisplayName  string
	ModelName    string
	CategoryName string
	Version      string
	Sort         string //asc or desc. Default behavior is asc
	OrderOn      string
	Limit        int //If 0 or  unspecified then all records are returned and limit is not used
	Offset       int
	Annotations  string //When this query parameter is "true", only components with the "isAnnotation" property set to true are returned. When this query parameter is "false", all components except those considered to be annotation components are returned. Any other value of the query parameter results in both annotations as well as non-annotation models being returned.
}

type componentDefinitionWithModel struct {
	v1beta1.ComponentDefinitionDB
	ModelDB    v1beta1.ModelDB // acoount for overridn fields
	CategoryDB v1beta1.CategoryDB
	HostsDB    v1beta1.Hostv1beta1
}

// Create the filter from map[string]interface{}
func (cf *ComponentFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func (componentFilter *ComponentFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	// componentFilter, err := utils.Cast[ComponentFilter](f)
	// if err != nil {
	// 	return nil, 0, 0, err
	// }

	countUniqueComponents := func(components []componentDefinitionWithModel) int {
		set := make(map[string]struct{})
		for _, compWithModel := range components {
			key := compWithModel.Component.Kind + "@" + compWithModel.Component.Version + "@" + compWithModel.ModelDB.Name + "@" + compWithModel.ModelDB.Version
			if _, ok := set[key]; !ok {
				set[key] = struct{}{}
			}
		}
		return len(set)
	}
	var componentDefinitionsWithModel []componentDefinitionWithModel
	finder := db.Model(&v1beta1.ComponentDefinitionDB{}).
		Select("component_definition_dbs.*, model_dbs.*,category_dbs.*").
		Joins("JOIN model_dbs ON component_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN hosts ON models.registrant_id = hosts.id")
		//

	if componentFilter.Greedy {
		if componentFilter.Name != "" && componentFilter.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.component->>'kind' LIKE ? OR display_name LIKE ?", "%"+componentFilter.Name+"%", componentFilter.DisplayName+"%")
		} else if componentFilter.Name != "" {
			finder = finder.Where("component_definition_dbs.component->>'kind' LIKE ?", "%"+componentFilter.Name+"%")
		} else if componentFilter.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.display_name LIKE ?", "%"+componentFilter.DisplayName+"%")
		}
	} else {
		if componentFilter.Name != "" {
			finder = finder.Where("component_definition_dbs.component->>'kind'  = ?", componentFilter.Name)
		}
		if componentFilter.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.display_name = ?", componentFilter.DisplayName)
		}
	}

	if componentFilter.ModelName != "" && componentFilter.ModelName != "all" {
		finder = finder.Where("model_dbs.name = ?", componentFilter.ModelName)
	}

	if componentFilter.Annotations == "true" {
		finder = finder.Where("component_definition_dbs.metadata->>'isAnnotation' = true")
	} else if componentFilter.Annotations == "false" {
		finder = finder.Where("component_definition_dbs.metadata->>'isAnnotation' = false")
	}

	if componentFilter.APIVersion != "" {
		finder = finder.Where("component_definition_dbs.component->>'version'  = ?", componentFilter.APIVersion)
	}
	if componentFilter.CategoryName != "" {
		finder = finder.Where("category_dbs.name = ?", componentFilter.CategoryName)
	}
	if componentFilter.Version != "" {
		finder = finder.Where("model_dbs.version = ?", componentFilter.Version)
	}

	var count int64
	finder.Count(&count)
	finder = finder.Offset(componentFilter.Offset)
	if componentFilter.Limit != 0 {
		finder = finder.Limit(componentFilter.Limit)
	}
	err := finder.
		Scan(&componentDefinitionsWithModel).Error
	if err != nil {
		fmt.Println(err.Error()) //for debugging
	}

	defs := make([]entity.Entity, len(componentDefinitionsWithModel))
	// remove this when compoentdef and componetdefdb struct is consolidated.
	for _, cm := range componentDefinitionsWithModel {
		if componentFilter.Trim {
			cm.Component.Schema = ""
		}
		// Ensure correct reg is passed, rn it is dummy for sake of testing.
		// In the first query above where we do seelection i think there changes will be requrired, an when that two def and defDB structs are consolidated, using association and preload i think we can do.
		reg := cm.HostsDB
		cd := cm.ComponentDefinitionDB.GetComponentDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db), reg))
		defs = append(defs, &cd)
	}

	unique := countUniqueComponents(componentDefinitionsWithModel)

	return defs, count, unique, nil
}
