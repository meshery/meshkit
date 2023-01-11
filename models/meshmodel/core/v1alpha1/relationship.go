package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// https://docs.google.com/drawings/d/1_qzQ_YxvCWPYrOBcdqGMlMwfbsZx96SBuIkbn8TfKhU/edit?pli=1
// see RELATIONSHIPDEFINITIONS table in the diagram
// swagger:response RelationshipDefinition
// TODO: Add support for Model
type RelationshipDefinition struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Model     Model                  `json:"model"`
	Metadata  map[string]interface{} `json:"metadata" yaml:"metadata"`
	SubType   string                 `json:"subType" yaml:"subType" gorm:"subType"`
	Selectors map[string]interface{} `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time              `json:"-"`
	UpdatedAt time.Time              `json:"-"`
}

type RelationshipDefinitionDB struct {
	ID      uuid.UUID `json:"-"`
	ModelID uuid.UUID `json:"-" gorm:"modelID"`
	TypeMeta
	Metadata  []byte    `json:"metadata" yaml:"metadata"`
	SubType   string    `json:"subType" yaml:"subType"`
	Selectors []byte    `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// For now, only filtering by Kind and SubType are allowed.
// In the future, we will add support to query using `selectors` (using CUE)
// TODO: Add support for Model
type RelationshipFilter struct {
	Kind      string
	Greedy    bool //when set to true - instead of an exact match, kind will be prefix matched
	SubType   string
	Version   string
	ModelName string
	OrderOn   string
	Sort      string //asc or desc. Default behavior is asc
	Limit     int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset    int
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

func GetRelationships(db *database.Handler, f RelationshipFilter) (rs []RelationshipDefinition) {
	var rdb []RelationshipDefinitionDB
	if f.ModelName != "" {
		var models []Model
		_ = db.Where("name = ?", f.ModelName).Find(&models).Error
		finder := db.Model(&rdb)
		if f.OrderOn != "" {
			if f.Sort == "desc" {
				finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: f.OrderOn}, Desc: true})
			} else {
				finder = finder.Order(f.OrderOn)
			}
		}
		skipLimit := false
		if f.Limit == 0 {
			skipLimit = true
		}
		if f.Kind != "" {
			if f.Greedy {
				finder = finder.Where("kind LIKE ?", f.Kind+"%")
			} else {
				finder = finder.Where("kind = ?", f.Kind)
			}
		}
		_ = finder.Find(&rdb).Error
		for _, rel := range rdb {
			//TODO: Use model id as foreign key in above DB calls instead of making comparisons here
			if f.Offset == 0 {
				if skipLimit || f.Limit > 0 {
					for _, mod := range models {
						if mod.ID == rel.ModelID {
							rs = append(rs, rel.GetRelationshipDefinition(mod))
							f.Limit--
							continue
						}
					}
				}
			} else {
				f.Offset--
			}

		}
	} else if f.Kind != "" {
		finder := db.Model(&rdb)
		if f.Greedy {
			finder = finder.Where("kind LIKE ?", f.Kind+"%")
		} else {
			finder = finder.Where("kind = ?", f.Kind)
		}
		if f.OrderOn != "" {
			if f.Sort == "desc" {
				finder = finder.Order(fmt.Sprintf("%s DESC", f.OrderOn))
			} else {
				finder = finder.Order(f.OrderOn)
			}
		}
		if f.Limit != 0 {
			finder = finder.Limit(f.Limit)
		}
		if f.Offset != 0 {
			finder = finder.Offset(f.Offset)
		}
		_ = finder.Find(&rdb).Error
		for _, compdb := range rdb {
			var model Model
			db.First(&model, "id = ?", compdb.ModelID)
			comp := compdb.GetRelationshipDefinition(model)
			rs = append(rs, comp)
		}
	} else {
		finder := db.Model(&rdb)
		if f.Limit != 0 {
			finder = finder.Limit(f.Limit)
		}
		if f.Offset != 0 {
			finder = finder.Offset(f.Offset)
		}
		finder.Find(&rdb)
		for _, compdb := range rdb {
			var model Model
			db.First(&model, "id = ?", compdb.ModelID)
			comp := compdb.GetRelationshipDefinition(model)
			rs = append(rs, comp)
		}
	}

	if f.Version != "" {
		var vrel []RelationshipDefinition
		for _, r := range rs {
			if r.Model.Version == f.Version {
				vrel = append(vrel, r)
			}
		}
		return vrel
	}
	return
}

func (rdb *RelationshipDefinitionDB) GetRelationshipDefinition(m Model) (r RelationshipDefinition) {
	r.ID = rdb.ID
	r.TypeMeta = rdb.TypeMeta
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Metadata, &r.Metadata)
	if r.Selectors == nil {
		r.Selectors = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Selectors, &r.Selectors)
	r.SubType = rdb.SubType
	r.Kind = rdb.Kind
	r.Model = m
	return
}

func (r RelationshipDefinition) Type() types.CapabilityType {
	return types.RelationshipDefinition
}
func (r RelationshipDefinition) GetID() uuid.UUID {
	return r.ID
}

func CreateRelationship(db *database.Handler, r RelationshipDefinition) (uuid.UUID, error) {
	r.ID = uuid.New()
	tempModelID := uuid.New()
	byt, err := json.Marshal(r.Model)
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
		model = r.Model
		model.ID = modelID
		err = db.Create(&model).Error
		if err != nil {
			componentCreationLock.Unlock()
			return uuid.UUID{}, err
		}
	}
	componentCreationLock.Unlock()
	rdb := r.GetRelationshipDefinitionDB()
	rdb.ModelID = model.ID
	err = db.Create(&rdb).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return r.ID, err
}

func (r *RelationshipDefinition) GetRelationshipDefinitionDB() (rdb RelationshipDefinitionDB) {
	rdb.ID = r.ID
	rdb.TypeMeta = r.TypeMeta
	rdb.Metadata, _ = json.Marshal(r.Metadata)
	rdb.Selectors, _ = json.Marshal(r.Selectors)
	rdb.Kind = r.Kind
	rdb.SubType = r.SubType
	rdb.ModelID = r.Model.ID
	return
}
