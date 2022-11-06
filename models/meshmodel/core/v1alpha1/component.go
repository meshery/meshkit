package v1alpha1

import (
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
)

type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion"`
}

// use NewComponent function for instantiating
type Component struct {
	ID        uuid.UUID
	TypeMeta  `gorm:"embedded" yaml:"typemeta"`
	Format    string
	Metadata  ComponentMetadata `gorm:"-"`
	Schema    []byte            `gorm:"embedded" yaml:"schema"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func CreateComponent(db *database.Handler, c Component) (uuid.UUID, error) {
	c.ID = uuid.New()
	c.Metadata.ID = uuid.New()
	compMeta := c.Metadata
	err := db.Create(&compMeta).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	err = db.Create(&c).Error
	return c.ID, err
}
func GetComponents(db *database.Handler, f ComponentFilter) []Component {
	//Add logic
	return nil
}

type ComponentFilter struct {
	Name string
}

// Create the filter from map[string]interface{}
func (cf ComponentFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}
func (c Component) Type() types.CapabilityType {
	return types.ComponentDefinition
}

type ComponentMetadata struct {
	ID          uuid.UUID
	ComponentID uuid.UUID
	Model       string
	Version     string
	Category    string
	SubCategory string
	Metadata    []byte
}

func NewComponent(kind string, apiVersion string, format string, model string, version string, metadata []byte, schema []byte) (Component, ComponentMetadata) {
	comp := Component{}
	comp.ID = uuid.New()
	comp.APIVersion = apiVersion
	comp.Kind = kind
	comp.Format = format
	comp.Schema = schema

	compMeta := ComponentMetadata{}
	compMeta.ID = uuid.New()
	compMeta.ComponentID = comp.ID
	compMeta.Model = model
	compMeta.Version = version

	return comp, compMeta
}
