package v1beta1

import (
	"encoding/json"
	"time"

	"github.com/layer5io/meshkit/database"
	types "github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/utils"

	"github.com/google/uuid"
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

func (c ComponentDefinition) Type() types.EntityType {
	return types.ComponentDefinition
}

func (c ComponentDefinition) GetID() uuid.UUID {
	return c.ID
}

func (c *ComponentDefinition) Create(db *database.Handler) (uuid.UUID, error) {
	c.ID = uuid.New()
	mid, err := c.Model.Create(db)
	if err != nil {
		return uuid.UUID{}, err
	}

	if !utils.IsSchemaEmpty(c.Schema) {
		c.Metadata["hasInvalidSchema"] = true
	}
	cdb := c.GetComponentDefinitionDB()
	cdb.ModelID = mid
	err = db.Create(&cdb).Error
	return c.ID, err
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
