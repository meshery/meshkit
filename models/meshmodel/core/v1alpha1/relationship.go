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
	Selectors RelationshipSelector   `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time              `json:"-"`
	UpdatedAt time.Time              `json:"-"`
}
type RelationshipSelector struct {
	Allow struct {
		From []RelationshipKindModelFilter `json:"from"`
		To   []RelationshipKindModelFilter `json:"to"`
	} `json:"allow"`

	Deny struct {
		From []RelationshipKindModelFilter `json:"from"`
		To   []RelationshipKindModelFilter `json:"to"`
	} `json:"deny"`
}

type RelationshipKindModelFilter struct {
	Kind  string `json:"kind"`
	Model string `json:"model"`
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
	Kind        string
	Greedy      bool //when set to true - instead of an exact match, kind will be prefix matched
	SubType     string
	Version     string
	AllowedFrom []RelationshipKindModelFilter
	AllowedTo   []RelationshipKindModelFilter
	DeniedFrom  []RelationshipKindModelFilter
	DeniedTo    []RelationshipKindModelFilter
	ModelName   string
	OrderOn     string
	Sort        string //asc or desc. Default behavior is asc
	Limit       int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset      int
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}
func GetMeshModelRelationship(db *database.Handler, f RelationshipFilter) (r []RelationshipDefinition) {
	type componentDefinitionWithModel struct {
		RelationshipDefinitionDB
		Model
	}
	var componentDefinitionsWithModel []componentDefinitionWithModel
	finder := db.Model(&RelationshipDefinitionDB{}).
		Select("relationship_definition_dbs.*, models.*").
		Joins("JOIN models ON relationship_definition_dbs.model_id = models.id") //
	if f.Kind != "" {
		if f.Greedy {
			finder = finder.Where("relationship_definition_dbs.kind LIKE ?", f.Kind)
		} else {
			finder = finder.Where("relationship_definition_dbs.kind = ?", f.Kind)
		}
	}
	if f.SubType != "" {
		finder = finder.Where("relationship_definition_dbs.sub_type = ?", f.SubType)
	}
	if f.ModelName != "" {
		finder = finder.Where("models.name = ?", f.ModelName)
	}
	if f.Version != "" {
		finder = finder.Where("models.version = ?", f.Version)
	}
	if f.OrderOn != "" {
		if f.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: f.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(f.OrderOn)
		}
	}
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
		rd := cm.RelationshipDefinitionDB.GetRelationshipDefinition(cm.Model)
		if f.AllowedFrom != nil && !matchRelationship(f.AllowedFrom, rd.Selectors.Allow.From) {
			continue
		}
		if f.AllowedTo != nil && !matchRelationship(f.AllowedTo, rd.Selectors.Allow.To) {
			continue
		}
		r = append(r, rd)
	}
	return r
}

// Checks if all the kind-model pair passed in "src" are present inside "tocheck" or not
func matchRelationship(src []RelationshipKindModelFilter, tocheck []RelationshipKindModelFilter) bool {
	counter := len(src)
	for _, t := range tocheck {
		for _, s := range src {
			if t.Kind == s.Kind && t.Model == s.Model {
				counter--
			}
			if counter == 0 {
				return true
			}
		}
	}
	if counter == 0 {
		return true
	}
	return false
}
func (rdb *RelationshipDefinitionDB) GetRelationshipDefinition(m Model) (r RelationshipDefinition) {
	r.ID = rdb.ID
	r.TypeMeta = rdb.TypeMeta
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Metadata, &r.Metadata)
	_ = json.Unmarshal(rdb.Selectors, &r.Selectors)
	r.SubType = rdb.SubType
	r.Kind = rdb.Kind
	r.Model = m
	return
}

func (r RelationshipDefinition) Type() types.CapabilityType {
	return types.RelationshipDefinition
}
func (r RelationshipDefinition) Doc(f DocFormat, db *database.Handler) string {
	//TODO: add doc
	return ""
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
	modelCreationLock.Lock()
	err = db.First(&model, "id = ?", modelID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if model.ID == tempModelID || err == gorm.ErrRecordNotFound { //The model is already not present and needs to be inserted
		model = r.Model
		model.ID = modelID
		err = db.Create(&model).Error
		if err != nil {
			modelCreationLock.Unlock()
			return uuid.UUID{}, err
		}
	}
	modelCreationLock.Unlock()
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
