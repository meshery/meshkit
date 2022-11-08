package v1alpha1

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
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

// use NewComponent function for instantiating
type ComponentDefinition struct {
	ID        uuid.UUID `json:"-"`
	TypeMeta  `gorm:"embedded" yaml:"typemeta"`
	Format    ComponentFormat   `gorm:"format"`
	Metadata  ComponentMetadata `gorm:"-"`
	Schema    string            `gorm:"embedded" yaml:"schema"`
	CreatedAt time.Time         `json:"-"`
	UpdatedAt time.Time         `json:"-"`
}
type ComponentDefinitionDB struct {
	ID        uuid.UUID `json:"-"`
	TypeMeta  `yaml:"typemeta"`
	Format    ComponentFormat     `gorm:"format"`
	Metadata  ComponentMetadataDB `gorm:"-"`
	Schema    string              `yaml:"schema"`
	CreatedAt time.Time           `json:"-"`
	UpdatedAt time.Time           `json:"-"`
}

func (c ComponentDefinition) Type() types.CapabilityType {
	return types.ComponentDefinition
}

func CreateComponent(db *database.Handler, c ComponentDefinition) (uuid.UUID, error) {
	cdb := ComponentDefinitionDB{}
	c.ID = uuid.New()
	c.Metadata.ID = uuid.New()
	c.Metadata.ComponentID = c.ID
	compMetaDB := ComponentMetadataDB{}
	compMetaDB.FromComponentMetadata(c.Metadata)
	cdb.FromComponentMetadata(c)
	err := db.Create(&cdb).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	err = db.Create(&compMetaDB).Error
	return c.ID, err
}

// TODO: Code duplication in below function, minor refactor needed
func GetComponents(db *database.Handler, f ComponentFilter) (c []ComponentDefinition) {
	if f.ModelName != "" {
		var metas []ComponentMetadataDB
		_ = db.Where("model = ?", f.ModelName).Find(&metas).Error
		var ids []uuid.UUID
		mapIDsToComponentsMetadata := make(map[uuid.UUID]*ComponentMetadataDB)
		for _, m := range metas {
			ids = append(ids, m.ComponentID)
			mapIDsToComponentsMetadata[m.ComponentID] = &m
		}
		var ctemp []ComponentDefinitionDB
		if f.Name == "" {
			_ = db.Where("id IN ?", ids).Find(&ctemp).Error
		} else {
			_ = db.Where("id IN ?", ids).Where("kind = ?", f.Name).Find(&ctemp).Error
		}
		for _, comp := range ctemp {
			comp.Metadata = *mapIDsToComponentsMetadata[comp.ID]
			c = append(c, comp.ToComponent())
		}
		return
	}

	if f.Name != "" {
		var metas []ComponentMetadataDB
		_ = db.Find(&metas).Error
		var ids []uuid.UUID
		mapIDsToComponentsMetadata := make(map[uuid.UUID]*ComponentMetadataDB)
		for _, m := range metas {
			ids = append(ids, m.ComponentID)
			mapIDsToComponentsMetadata[m.ComponentID] = &m
		}
		var ctemp []ComponentDefinitionDB
		_ = db.Where("id IN ?", ids).Where("kind = ?", f.Name).Find(&ctemp).Error
		for _, comp := range ctemp {
			comp.Metadata = *mapIDsToComponentsMetadata[comp.ID]
			c = append(c, comp.ToComponent())
		}
		return
	}
	var metas []ComponentMetadataDB
	_ = db.Find(&metas).Error
	var ids []uuid.UUID
	mapIDsToComponentsMetadata := make(map[uuid.UUID]*ComponentMetadataDB)
	for _, m := range metas {
		ids = append(ids, m.ComponentID)
		mapIDsToComponentsMetadata[m.ComponentID] = &m
	}
	var ctemp []ComponentDefinitionDB
	if f.Name == "" {
		_ = db.Where("id IN ?", ids).Find(&ctemp).Error
	} else {
		_ = db.Where("id IN ?", ids).Find(&ctemp).Error
	}
	for _, compdb := range ctemp {
		compdb.Metadata = *mapIDsToComponentsMetadata[compdb.ID]
		c = append(c, compdb.ToComponent())
	}
	return
}

type ComponentFilter struct {
	Name      string
	ModelName string
}

// Create the filter from map[string]interface{}
func (cf *ComponentFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

type ComponentMetadata struct {
	ID          uuid.UUID `json:"-"`
	ComponentID uuid.UUID `json:"-"`
	Model       string
	Version     string
	Category    string
	SubCategory string
	Metadata    map[string]interface{}
}

// This struct is internal to the system
type ComponentMetadataDB struct {
	ID          uuid.UUID
	ComponentID uuid.UUID
	Model       string
	Version     string
	Category    string
	SubCategory string
	Metadata    []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (cmd *ComponentMetadataDB) ToComponentMetadata() (c ComponentMetadata) {
	c.ID = cmd.ID
	c.ComponentID = cmd.ComponentID
	c.Model = cmd.Model
	c.Version = cmd.Version
	c.Category = cmd.Category
	c.SubCategory = cmd.SubCategory
	_ = json.Unmarshal(cmd.Metadata, &c.Metadata)
	return
}
func (cmd *ComponentMetadataDB) FromComponentMetadata(c ComponentMetadata) {
	cmd.ID = c.ID
	cmd.ComponentID = c.ComponentID
	cmd.Model = c.Model
	cmd.Version = c.Version
	cmd.Category = c.Category
	cmd.SubCategory = c.SubCategory

	cmd.Metadata, _ = json.Marshal(c.Metadata)
	return
}

func (cmd *ComponentDefinitionDB) ToComponent() (c ComponentDefinition) {
	c.ID = cmd.ID
	c.TypeMeta = cmd.TypeMeta
	c.Format = cmd.Format
	c.Metadata = cmd.Metadata.ToComponentMetadata()
	c.Schema = cmd.Schema
	return
}
func (cmd *ComponentDefinitionDB) FromComponentMetadata(c ComponentDefinition) {
	cmd.ID = c.ID
	cmd.TypeMeta = c.TypeMeta
	cmd.Format = c.Format
	cmd.Metadata.FromComponentMetadata(c.Metadata)
	cmd.Schema = c.Schema
	return
}
