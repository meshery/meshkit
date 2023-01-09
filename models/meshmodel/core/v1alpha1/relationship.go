package v1alpha1

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"gorm.io/gorm"
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
	SubType   string
	ModelName string
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

func GetRelationships(db *database.Handler, f RelationshipFilter) (rs []RelationshipDefinition) {
	var rdb []RelationshipDefinitionDB
	// GORM takes care of drafting the correct SQL
	// https://gorm.io/docs/query.html#Struct-amp-Map-Conditions
	_ = db.Where(&RelationshipDefinitionDB{SubType: f.SubType, TypeMeta: TypeMeta{Kind: f.Kind}}).Find(&rdb)
	for _, reldb := range rdb {
		var mod Model
		if f.ModelName != "" {
			db.Where("id = ?", reldb.ModelID).Where("name = ?", f.ModelName).Find(&mod)
		}
		if mod.Name != "" { //relationships with a valid model name will be returned
			rel := reldb.GetRelationshipDefinition(mod)
			rs = append(rs, rel)
		}
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
