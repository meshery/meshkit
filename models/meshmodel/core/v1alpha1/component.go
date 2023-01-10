package v1alpha1

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"gorm.io/gorm"
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
	ID uuid.UUID `json:"-"`
	TypeMeta
	DisplayName string                 `json:"display-name" gorm:"display-name"`
	Format      ComponentFormat        `json:"format" yaml:"format"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata"`
	Model       Model                  `json:"model"`
	Schema      string                 `json:"schema" yaml:"schema"`
	CreatedAt   time.Time              `json:"-"`
	UpdatedAt   time.Time              `json:"-"`
}
type ComponentDefinitionDB struct {
	ID      uuid.UUID `json:"-"`
	ModelID uuid.UUID `json:"-" gorm:"modelID"`
	TypeMeta
	DisplayName string          `json:"display-name" gorm:"display-name"`
	Format      ComponentFormat `json:"format" yaml:"format"`
	Metadata    []byte          `json:"metadata" yaml:"metadata"`
	Schema      string          `json:"schema" yaml:"schema"`
	CreatedAt   time.Time       `json:"-"`
	UpdatedAt   time.Time       `json:"-"`
}
type Model struct {
	ID          uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	DisplayName string    `json:"display-name" gorm:"display-name"`
	Category    string    `json:"category"`
	SubCategory string    `json:"sub-category"`
}

func (c ComponentDefinition) Type() types.CapabilityType {
	return types.ComponentDefinition
}
func (c ComponentDefinition) GetID() uuid.UUID {
	return c.ID
}

var componentCreationLock sync.Mutex

func CreateComponent(db *database.Handler, c ComponentDefinition) (uuid.UUID, error) {
	c.ID = uuid.New()
	tempModelID := uuid.New()
	byt, err := json.Marshal(c.Model)
	if err != nil {
		return uuid.UUID{}, err
	}
	modelID := uuid.NewSHA1(uuid.UUID{}, byt)
	var model Model
	componentCreationLock.Lock()
	err = db.First(&model, "id = ?", modelID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if model.ID == tempModelID || err == gorm.ErrRecordNotFound { //The model is already not present and needs to be inserted
		model = c.Model
		model.ID = modelID
		err = db.Create(&model).Error
		if err != nil {
			componentCreationLock.Unlock()
			return uuid.UUID{}, err
		}
	}
	componentCreationLock.Unlock()
	cdb := c.GetComponentDefinitionDB()
	cdb.ModelID = model.ID
	err = db.Create(&cdb).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return c.ID, err
}

// TODO: Optimize the below queries with joins
func GetComponents(db *database.Handler, f ComponentFilter) (c []ComponentDefinition) {
	var cdb []ComponentDefinitionDB
	if f.ModelName != "" {
		var models []Model
		_ = db.Where("name = ?", f.ModelName).Find(&models).Error
		finder := db.Model(&cdb)
		if f.Sort {
			finder = finder.Order("kind")
		}
		skipLimit := false
		if f.Limit == 0 {
			skipLimit = true
		}
		if f.Name != "" {
			if f.Greedy {
				finder = finder.Where("kind LIKE ?", f.Name+"%")
			} else {
				finder = finder.Where("kind = ?", f.Name)
			}
		}
		_ = finder.Find(&cdb).Error
		for _, comp := range cdb {
			//TODO: Use model id as foreign key in above DB calls instead of making comparisons here
			if f.Offset == 0 {
				if skipLimit || f.Limit > 0 {
					for _, mod := range models {
						if mod.ID == comp.ModelID {
							c = append(c, comp.GetComponentDefinition(mod))
							f.Limit--
							continue
						}
					}
				}
			} else {
				f.Offset--
			}

		}
	} else if f.Name != "" {
		finder := db.Model(&cdb)
		if f.Greedy {
			finder = finder.Where("kind LIKE ?", f.Name+"%")
		} else {
			finder = finder.Where("kind = ?", f.Name)
		}
		if f.Sort {
			finder = finder.Order("kind")
		}
		if f.Limit != 0 {
			finder = finder.Limit(f.Limit)
		}
		if f.Offset != 0 {
			finder = finder.Offset(f.Offset)
		}
		_ = finder.Find(&cdb).Error
		for _, compdb := range cdb {
			var model Model
			db.First(&model, "id = ?", compdb.ModelID)
			comp := compdb.GetComponentDefinition(model)
			c = append(c, comp)
		}
	} else {
		finder := db.Model(&cdb)
		if f.Limit != 0 {
			finder = finder.Limit(f.Limit)
		}
		if f.Offset != 0 {
			finder = finder.Offset(f.Offset)
		}
		finder.Find(&cdb)
		for _, compdb := range cdb {
			var model Model
			db.First(&model, "id = ?", compdb.ModelID)
			comp := compdb.GetComponentDefinition(model)
			c = append(c, comp)
		}
	}

	if f.Version != "" {
		var vcomp []ComponentDefinition
		for _, comp := range c {
			if comp.Model.Version == f.Version {
				vcomp = append(vcomp, comp)
			}
		}
		return vcomp
	}
	return
}

type ComponentFilter struct {
	Name      string
	Greedy    bool //when set to true - instead of an exact match, name will be prefix matched
	ModelName string
	Version   string
	Sort      bool //when set to true -  the returned entities will be sorted on Name
	Limit     int  //If 0 or  unspecified then all records are returned and limit is not used
	Offset    int
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
