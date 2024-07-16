package v1beta1

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"gorm.io/gorm/clause"
)

type ComponentFilter struct {
    Id           string
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
	ComponentDefinitionDB v1beta1.ComponentDefinition `gorm:"embedded"`
	ModelDB               v1beta1.Model               `gorm:"embedded"`
	CategoryDB            v1beta1.Category            `gorm:"embedded"`
	HostsDB               v1beta1.Host                `gorm:"embedded"`
}

func (cf *ComponentFilter) GetById(db *database.Handler) (entity.Entity, error) {
    c := &v1beta1.ComponentDefinition{}
    err := db.First(c, "id = ?", cf.Id).Error
	if err != nil {
		return nil, err
	}
    return  c, err
}

// Create the filter from map[string]interface{}
func (cf *ComponentFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func countUniqueComponents(components []componentDefinitionWithModel) int {
	set := make(map[string]struct{})
	for _, compWithModel := range components {
		key := compWithModel.ComponentDefinitionDB.Component.Kind + "@" + compWithModel.ComponentDefinitionDB.Component.Version + "@" + compWithModel.ModelDB.Name + "@" + compWithModel.ModelDB.Model.Version
		if _, ok := set[key]; !ok {
			set[key] = struct{}{}
		}
	}
	return len(set)
}

func (componentFilter *ComponentFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	var componentDefinitionsWithModel []componentDefinitionWithModel
	finder := db.Model(&v1beta1.ComponentDefinition{}).
		Select("component_definition_dbs.*, model_dbs.*,category_dbs.*, hosts.*").
		Joins("JOIN model_dbs ON component_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN hosts ON hosts.id = model_dbs.host_id")
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
		finder = finder.Where("model_dbs.model->>'version' = ?", componentFilter.Version)
	}

	if componentFilter.OrderOn != "" {
		if componentFilter.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: componentFilter.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(componentFilter.OrderOn)
		}
	} else {
		finder = finder.Order("display_name")
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
		return nil, 0, 0, err
	}

	defs := make([]entity.Entity, 0, len(componentDefinitionsWithModel))

	for _, cm := range componentDefinitionsWithModel {
		if componentFilter.Trim {
			cm.ComponentDefinitionDB.Component.Schema = ""
		}

		reg := cm.HostsDB
		cd := cm.ComponentDefinitionDB
		cd.Model = cm.ModelDB
		cd.Model.Category = cm.CategoryDB
		cd.Model.Registrant = reg
		defs = append(defs, &cd)
	}

	uniqueCount := countUniqueComponents(componentDefinitionsWithModel)

	return defs, count, uniqueCount, nil
}
