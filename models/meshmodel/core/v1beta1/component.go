package v1beta1

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/layer5io/meshkit/utils"

	"github.com/google/uuid"
)

type VersionMeta struct {
	SchemaVersion string `json:"schemaVersion,omitempty" yaml:"schemaVersion"`
	Version       string `json:"version,omitempty" yaml:"version"`
}

type TypeMeta struct {
	Kind    string `json:"kind,omitempty" yaml:"kind"`
	Version string `json:"version,omitempty" yaml:"version"`
}

type ComponentFormat string

const (
	JSON ComponentFormat = "JSON"
	YAML ComponentFormat = "YAML"
	CUE  ComponentFormat = "CUE"
)

type component struct {
	TypeMeta
	Schema string `json:"schema,omitempty" yaml:"schema"`
}

type componentDefinitionWithModel struct {
	ComponentDefinitionDB
	ModelDB ModelDB // acoount for overridn fields	
	CategoryDB CategoryDB
}

// swagger:response ComponentDefinition
// use NewComponent function for instantiating
type ComponentDefinition struct {
	ID uuid.UUID `json:"id,omitempty"`
	VersionMeta
	DisplayName string                 `json:"displayName" gorm:"displayName"`
	Description string                 `json:"description" gorm:"description"`
	Format      ComponentFormat        `json:"format" yaml:"format"`
	Model       Model                  `json:"model"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata"`
	// component corresponds to the specifications of underlying entity eg: Pod/Deployment....
	Component component `json:"component,omitempty" yaml:"component"`
}

type ComponentDefinitionDB struct {
	ID uuid.UUID `json:"id"`
	VersionMeta
	DisplayName string          `json:"displayName" gorm:"displayName"`
	Description string          `json:"description" gorm:"description"`
	Format      ComponentFormat `json:"format" yaml:"format"`
	ModelID     uuid.UUID       `json:"-" gorm:"index:idx_component_definition_dbs_model_id,column:modelID"`
	Metadata    []byte          `json:"metadata" yaml:"metadata"`
	Component   component       `json:"component,omitempty" yaml:"component" gorm:"component"`
}

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

func (c ComponentDefinition) Type() entity.EntityType {
	return entity.ComponentDefinition
}

func (c ComponentDefinition) GetID() uuid.UUID {
	return c.ID
}

func (componentFilter *ComponentFilter) Get(db *database.Handler) ([]ComponentDefinition, int64, int, error) {
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
	finder := db.Model(&ComponentDefinitionDB{}).
		Select("component_definition_dbs.*, model_dbs.*,category_dbs.*").
		Joins("JOIN model_dbs ON component_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id") //

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

	defs := make([]ComponentDefinition, len(componentDefinitionsWithModel))
	// remove this when compoentdef and componetdefdb struct is consolidated.
	for _, cm := range componentDefinitionsWithModel {
		if componentFilter.Trim {
			cm.Component.Schema = ""
		}
		// Ensure correct reg is passed, rn it is dummy for sake of testing.
		// In the first query above where we do seelection i think there changes will be requrired, an when that two def and defDB structs are consolidated, using association and preload i think we can do.
		reg := registry.Hostv1beta1{}
		defs = append(defs, cm.ComponentDefinitionDB.GetComponentDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db), reg)))
	}

	unique := countUniqueComponents(componentDefinitionsWithModel)

	return defs, count, unique, nil
}

func (c *ComponentDefinition) Create(db *database.Handler) (uuid.UUID, error) {
	c.ID = uuid.New()
	mid, err := c.Model.Create(db)
	if err != nil {
		return uuid.UUID{}, err
	}

	if !utils.IsSchemaEmpty(c.Component.Schema) {
		c.Metadata["hasInvalidSchema"] = true
	}
	cdb := c.GetComponentDefinitionDB()
	cdb.ModelID = mid
	err = db.Create(&cdb).Error
	return c.ID, err
}

func (m *ComponentDefinition) UpdateStatus(db database.Handler, status entity.EntityStatus) error {
	return nil
}

func (c *ComponentDefinition) GetComponentDefinitionDB() (cmd ComponentDefinitionDB) {
	// cmd.ID = c.ID id will be assigned by the database itself don't use this, as it will be always uuid.nil, because id is not known when comp gets generated.
	// While database creates an entry with valid primary key but to avoid confusion, it is disabled and accidental assignment of custom id.
	cmd.VersionMeta = c.VersionMeta
	cmd.DisplayName = c.DisplayName
	cmd.Description = c.Description
	cmd.Format = c.Format
	cmd.ModelID = c.Model.ID
	cmd.Metadata, _ = json.Marshal(c.Metadata)
	cmd.Component = c.Component
	return
}

func (c ComponentDefinition) WriteComponentDefinition(componentDirPath string) error {
	componentPath := filepath.Join(componentDirPath, c.Component.Kind+".json")
	err := utils.WriteJSONToFile[ComponentDefinition](componentPath, c)
	return err
}

func (cmd *ComponentDefinitionDB) GetComponentDefinition(model Model) (c ComponentDefinition) {
	c.ID = cmd.ID
	c.VersionMeta = cmd.VersionMeta
	c.DisplayName = cmd.DisplayName
	c.Description = cmd.Description
	c.Format = cmd.Format
	c.Model = model
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(cmd.Metadata, &c.Metadata)
	c.Component = cmd.Component
	return
}
