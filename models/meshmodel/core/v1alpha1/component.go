package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"gorm.io/gorm/clause"
)

type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion"`
}
type ComponentFormat string

const (
	JSON ComponentFormat = "JSON"
	YAML ComponentFormat = "YAML"
	CUE  ComponentFormat = "CUE"
)

// swagger:response ComponentDefinition
// use NewComponent function for instantiating
type ComponentDefinition struct {
	ID uuid.UUID `json:"id,omitempty"`
	TypeMeta
	DisplayName     string                 `json:"displayName" gorm:"displayName"`
	Format          ComponentFormat        `json:"format" yaml:"format"`
	HostName        string                 `json:"hostname,omitempty"`
	HostID          uuid.UUID              `json:"hostID,omitempty"`
	DisplayHostName string                 `json:"displayhostname,omitempty"`
	Metadata        map[string]interface{} `json:"metadata" yaml:"metadata"`
	Model           Model                  `json:"model"`
	Schema          string                 `json:"schema,omitempty" yaml:"schema"`
	CreatedAt       time.Time              `json:"-"`
	UpdatedAt       time.Time              `json:"-"`
}
type ComponentDefinitionDB struct {
	ID      uuid.UUID `json:"id"`
	ModelID uuid.UUID `json:"-" gorm:"index:idx_component_definition_dbs_model_id,column:modelID"`
	TypeMeta
	DisplayName string          `json:"displayName" gorm:"displayName"`
	Format      ComponentFormat `json:"format" yaml:"format"`
	Metadata    []byte          `json:"metadata" yaml:"metadata"`
	Schema      string          `json:"schema,omitempty" yaml:"schema"`
	CreatedAt   time.Time       `json:"-"`
	UpdatedAt   time.Time       `json:"-"`
}

func (c ComponentDefinition) Type() types.CapabilityType {
	return types.ComponentDefinition
}
func (c ComponentDefinition) GetID() uuid.UUID {
	return c.ID
}
func emptySchemaCheck(schema string) (valid bool) {
	if schema == "" {
		return
	}
	m := make(map[string]interface{})
	_ = json.Unmarshal([]byte(schema), &m)
	if m["properties"] == nil {
		return
	}
	valid = true
	return
}
func CreateComponent(db *database.Handler, c ComponentDefinition) (uuid.UUID, uuid.UUID, error) {
	c.ID = uuid.New()
	mid, err := CreateModel(db, c.Model)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}

	if !emptySchemaCheck(c.Schema) {
		c.Metadata["hasInvalidSchema"] = true
	}
	cdb := c.GetComponentDefinitionDB()
	cdb.ModelID = mid
	err = db.Create(&cdb).Error
	return c.ID, mid, err
}
func GetMeshModelComponents(db *database.Handler, f ComponentFilter) (c []ComponentDefinition, count int64, unique int) {
	type componentDefinitionWithModel struct {
		ComponentDefinitionDB
		ModelDB
		CategoryDB
	}

	countUniqueComponents := func(components []componentDefinitionWithModel) int {
		set := make(map[string]struct{})
		for _, model := range components {
			key := model.ComponentDefinitionDB.Kind + "@" + model.APIVersion + "@" + model.ModelDB.Name + "@" + model.ModelDB.Version
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

	if f.Greedy {
		if f.Name != "" && f.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.kind LIKE ? OR display_name LIKE ?", "%"+f.Name+"%", f.DisplayName+"%")
		} else if f.Name != "" {
			finder = finder.Where("component_definition_dbs.kind LIKE ?", "%"+f.Name+"%")
		} else if f.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.display_name LIKE ?", "%"+f.DisplayName+"%")
		}
	} else {
		if f.Name != "" {
			finder = finder.Where("component_definition_dbs.kind = ?", f.Name)
		}
		if f.DisplayName != "" {
			finder = finder.Where("component_definition_dbs.display_name = ?", f.DisplayName)
		}
	}
	if f.ModelName != "" && f.ModelName != "all" {
		finder = finder.Where("model_dbs.name = ?", f.ModelName)
	}

	if f.Annotations == "true" {
		finder = finder.Where("component_definition_dbs.metadata->>'isAnnotation' = true")
	} else if f.Annotations == "false" {
		finder = finder.Where("component_definition_dbs.metadata->>'isAnnotation' = false")
	}

	if f.APIVersion != "" {
		finder = finder.Where("component_definition_dbs.api_version = ?", f.APIVersion)
	}
	if f.CategoryName != "" {
		finder = finder.Where("category_dbs.name = ?", f.CategoryName)
	}
	if f.Version != "" {
		finder = finder.Where("model_dbs.version = ?", f.Version)
	}
	if f.OrderOn != "" {
		if f.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: f.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(f.OrderOn)
		}
	}

	finder.Count(&count)

	finder = finder.Offset(f.Offset)
	if f.Limit != 0 {
		finder = finder.Limit(f.Limit)
	}
	err := finder.
		Scan(&componentDefinitionsWithModel).Error
	if err != nil {
		fmt.Println(err.Error()) //for debugging
	}
	for _, cm := range componentDefinitionsWithModel {
		if f.Trim {
			cm.Schema = ""
		}
		c = append(c, cm.ComponentDefinitionDB.GetComponentDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db))))
	}

	unique = countUniqueComponents(componentDefinitionsWithModel)

	return c, count, unique
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

// Create the filter from map[string]interface{}
func (cf *ComponentFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func (cmd *ComponentDefinitionDB) GetComponentDefinition(model Model) (c ComponentDefinition) {
	c.ID = cmd.ID
	c.TypeMeta = cmd.TypeMeta
	c.Format = cmd.Format
	c.DisplayName = cmd.DisplayName
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(cmd.Metadata, &c.Metadata)
	c.Schema = cmd.Schema
	c.Model = model
	return
}
func (c *ComponentDefinition) GetComponentDefinitionDB() (cmd ComponentDefinitionDB) {
	cmd.ID = c.ID
	cmd.TypeMeta = c.TypeMeta
	cmd.Format = c.Format
	cmd.Metadata, _ = json.Marshal(c.Metadata)
	cmd.DisplayName = c.DisplayName
	cmd.Schema = c.Schema
	return
}
