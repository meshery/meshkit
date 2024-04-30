package v1beta1

import (
	"fmt"
	"path/filepath"

	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/utils"
	"gorm.io/gorm/clause"

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

// Contains information as extracted from the core underlying component eg: Pod's apiVersion, kind and schema
type ComponentEntity struct {
	TypeMeta
	Schema string `json:"schema,omitempty" yaml:"schema"`
}

// swagger:response ComponentDefinition
type ComponentDefinition struct {
	ID uuid.UUID `json:"id"`
	VersionMeta
	DisplayName string                 `json:"displayName" gorm:"displayName"`
	Description string                 `json:"description" gorm:"description"`
	Format      ComponentFormat        `json:"format" yaml:"format"`
	ModelID     uuid.UUID              `json:"-" gorm:"index:idx_component_definition_dbs_model_id,column:model_id"`
	Model       Model                  `json:"model" gorm:"foreignKey:ModelID;references:ID"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata" gorm:"type:bytes;serializer:json"`
	Component   ComponentEntity        `json:"component,omitempty" yaml:"component" gorm:"type:bytes;serializer:json"`
}

func (c ComponentDefinition) TableName() string {
	return "component_definition_dbs"
}


func (c ComponentDefinition) Type() entity.EntityType {
	return entity.ComponentDefinition
}

func (c ComponentDefinition) GetID() uuid.UUID {
	return c.ID
}

func (c *ComponentDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, definition version: %s, name: %s, model: %s, version: %s", c.Type(), c.Version, c.DisplayName, c.Model.Name, c.Model.Version)
}

func (c *ComponentDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	c.ID = uuid.New()

	isAnnotation, _ := c.Metadata["isAnnotation"].(bool)

	if c.Component.Schema == "" && !isAnnotation { //For components which has an empty schema and is not an annotation, return error
		// return ErrEmptySchema()
		return uuid.Nil, nil
	}

	mid, err := c.Model.Create(db, hostID)
	if err != nil {
		return uuid.UUID{}, err
	}

	if !utils.IsSchemaEmpty(c.Component.Schema) {
		c.Metadata["hasInvalidSchema"] = true
	}

	c.ModelID = mid
	err = db.Omit(clause.Associations).Create(&c).Error
	return c.ID, err
}

func (m *ComponentDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	return nil
}

func (c ComponentDefinition) WriteComponentDefinition(componentDirPath string) error {
	if c.Component.Kind == "" {
		return nil
	}
	componentPath := filepath.Join(componentDirPath, c.Component.Kind+".json")
	err := utils.WriteJSONToFile[ComponentDefinition](componentPath, c)
	return err
}
