package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	types "github.com/layer5io/meshkit/models/meshmodel/entity"
	"gorm.io/gorm/clause"
)

// https://docs.google.com/drawings/d/1_qzQ_YxvCWPYrOBcdqGMlMwfbsZx96SBuIkbn8TfKhU/edit?pli=1
// see RELATIONSHIPDEFINITIONS table in the diagram
// swagger:response RelationshipDefinition
// TODO: Add support for Model
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

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}
func GetMeshModelRelationship(db *database.Handler, f RelationshipFilter) (r []RelationshipDefinition, count int64) {
	type componentDefinitionWithModel struct {
		RelationshipDefinitionDB
		ModelDB //nolint
		CategoryDB
	}
	var componentDefinitionsWithModel []componentDefinitionWithModel
	finder := db.Model(&RelationshipDefinitionDB{}).
		Select("relationship_definition_dbs.*, model_dbs.*").
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id"). //
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")           //
	if f.Kind != "" {
		if f.Greedy {
			finder = finder.Where("relationship_definition_dbs.kind LIKE ?", "%"+f.Kind+"%")
		} else {
			finder = finder.Where("relationship_definition_dbs.kind = ?", f.Kind)
		}
	}

	if f.RelationshipType != "" {
		finder = finder.Where("relationship_definition_dbs.type = ?", f.RelationshipType)
	}

	if f.SubType != "" {
		finder = finder.Where("relationship_definition_dbs.sub_type = ?", f.SubType)
	}
	if f.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", f.ModelName)
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
		r = append(r, cm.RelationshipDefinitionDB.GetRelationshipDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db))))
	}
	return r, count
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

func (r RelationshipDefinition) Type() types.EntityType {
	return types.RelationshipDefinition
}
func (r RelationshipDefinition) GetID() uuid.UUID {
	return r.ID
}

func CreateRelationship(db *database.Handler, r RelationshipDefinition) (uuid.UUID, uuid.UUID, error) {
	r.ID = uuid.New()
	mid, err := CreateModel(db, r.Model)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	rdb := r.GetRelationshipDefinitionDB()
	rdb.ModelID = mid
	err = db.Create(&rdb).Error
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	return r.ID, mid, err
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
