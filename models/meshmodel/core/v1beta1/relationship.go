package v1beta1

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	types "github.com/layer5io/meshkit/models/meshmodel/entity"
)

// Remove additional DB structs from all entites they are used just for marshaling & unmarshlling and tarcking modelID vs model(modelID when referenced in DB and full model when sending data, use foregin key references tag of gorm for that as used in MeshSync tables) when saving retrivng from database, insted use gorm' serailization tag as used in events.
type RelationshipDefinition struct {
	ID uuid.UUID `json:"id"`
	TypeMeta
	Model           Model                  `json:"model"`
	HostName        string                 `json:"hostname"`
	HostID          uuid.UUID              `json:"hostID"`
	DisplayHostName string                 `json:"displayhostname"`
	Metadata        map[string]interface{} `json:"metadata" yaml:"metadata"`
	// The property has been named RelationshipType instead of Type to avoid collision from Type() function, which enables support for dynamic type.
	// Though, the column name and the json representation is "type".
	RelationshipType string                   `json:"type" yaml:"type" gorm:"type"`
	SubType          string                   `json:"subType" yaml:"subType" gorm:"subType"`
	EvaluationQuery  string                   `json:"evaluationQuery" yaml:"evaluationQuery" gorm:"evaluationQuery"`
	Selectors        []map[string]interface{} `json:"selectors" yaml:"selectors"`
	CreatedAt        time.Time                `json:"-"`
	UpdatedAt        time.Time                `json:"-"`
}

type RelationshipDefinitionDB struct {
	ID      uuid.UUID `json:"id"`
	ModelID uuid.UUID `json:"-" gorm:"index:idx_relationship_definition_dbs_model_id,column:modelID"`
	TypeMeta
	Metadata []byte `json:"metadata" yaml:"metadata"`
	// The property has been named RelationshipType instead of Type to avoid collision from Type() function, which enables support for dynamic type.
	// Though, the column name and the json representation is "type".
	RelationshipType string    `json:"type" yaml:"type" gorm:"type"`
	SubType          string    `json:"subType" yaml:"subType"`
	EvaluationQuery  string    `json:"evaluationQuery" yaml:"evaluationQuery" gorm:"evaluationQuery"`
	Selectors        []byte    `json:"selectors" yaml:"selectors"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`
}

// For now, only filtering by Kind and SubType are allowed.
// In the future, we will add support to query using `selectors` (using CUE)
// TODO: Add support for Model
type RelationshipFilter struct {
	Kind             string
	Greedy           bool //when set to true - instead of an exact match, kind will be prefix matched
	SubType          string
	RelationshipType string
	Version          string
	ModelName        string
	OrderOn          string
	Sort             string //asc or desc. Default behavior is asc
	Limit            int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset           int
}

func (r RelationshipDefinition) Type() types.EntityType {
	return types.RelationshipDefinition
}
func (r RelationshipDefinition) GetID() uuid.UUID {
	return r.ID
}

func (r *RelationshipDefinition) Create(db *database.Handler) (uuid.UUID, error) {
	r.ID = uuid.New()
	mid, err := r.Model.Create(db)
	if err != nil {
		return uuid.UUID{}, err
	}
	rdb := r.GetRelationshipDefinitionDB()
	rdb.ModelID = mid
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
	rdb.RelationshipType = r.RelationshipType
	rdb.SubType = r.SubType
	rdb.ModelID = r.Model.ID
	rdb.EvaluationQuery = r.EvaluationQuery
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
		r.Selectors = []map[string]interface{}{}
	}
	_ = json.Unmarshal(rdb.Selectors, &r.Selectors)
	r.RelationshipType = rdb.RelationshipType
	r.SubType = rdb.SubType
	r.Kind = rdb.Kind
	r.Model = m
	r.EvaluationQuery = rdb.EvaluationQuery
	return
}
