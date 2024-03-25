package v1alpha2

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/layer5io/meshkit/utils"
	"gorm.io/gorm/clause"
)

// Remove additional DB structs from all entites they are used just for marshaling & unmarshlling and tarcking modelID vs model(modelID when referenced in DB and full model when sending data, use foregin key references tag of gorm for that as used in MeshSync tables) when saving retrivng from database, insted use gorm' serailization tag as used in events.
type RelationshipDefinition struct {
	ID uuid.UUID `json:"id"`
	v1beta1.VersionMeta
	Kind string `json:"kind,omitempty" yaml:"kind"`
	// The property has been named RelationshipType instead of Type to avoid collision from Type() function, which enables support for dynamic type.
	// Though, the column name and the json representation is "type".
	RelationshipType string                   `json:"type" yaml:"type" gorm:"type"`
	SubType          string                   `json:"subType" yaml:"subType" gorm:"subType"`
	EvaluationQuery  string                   `json:"evaluationQuery" yaml:"evaluationQuery" gorm:"evaluationQuery"`
	Metadata         map[string]interface{}   `json:"metadata" yaml:"metadata"`
	Model            v1beta1.Model            `json:"model"`
	Selectors        []map[string]interface{} `json:"selectors" yaml:"selectors"`
}

type RelationshipDefinitionDB struct {
	ID uuid.UUID `json:"id"`
	v1beta1.VersionMeta
	Kind string `json:"kind,omitempty" yaml:"kind"`
	// The property has been named RelationshipType instead of Type to avoid collision from Type() function, which enables support for dynamic type.
	// Though, the column name and the json representation is "type".
	RelationshipType string    `json:"type" yaml:"type" gorm:"type"`
	SubType          string    `json:"subType" yaml:"subType"`
	EvaluationQuery  string    `json:"evaluationQuery" yaml:"evaluationQuery" gorm:"evaluationQuery"`
	Metadata         []byte    `json:"metadata" yaml:"metadata"`
	ModelID          uuid.UUID `json:"-" gorm:"index:idx_relationship_definition_dbs_model_id,column:modelID"`
	Selectors        []byte    `json:"selectors" yaml:"selectors"`
}

type relationshipDefinitionWithModel struct {
	RelationshipDefinitionDB
	v1beta1.ModelDB //nolint
	v1beta1.CategoryDB
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

func (r RelationshipDefinition) Type() entity.EntityType {
	return entity.RelationshipDefinition
}
func (r RelationshipDefinition) GetID() uuid.UUID {
	return r.ID
}

func (r *RelationshipDefinition) Get(db *database.Handler, f entity.Filter) ([]RelationshipDefinition, int64, int, error) {
	relationshipFilter, err := utils.Cast[RelationshipFilter](f)
	if err != nil {
		return nil, 0, 0, err
	}

	var relationshipDefinitionsWithModel []relationshipDefinitionWithModel
	finder := db.Model(&RelationshipDefinitionDB{}).
		Select("relationship_definition_dbs.*, model_dbs.*").
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id"). //
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")           //
	if relationshipFilter.Kind != "" {
		if relationshipFilter.Greedy {
			finder = finder.Where("relationship_definition_dbs.kind LIKE ?", "%"+relationshipFilter.Kind+"%")
		} else {
			finder = finder.Where("relationship_definition_dbs.kind = ?", relationshipFilter.Kind)
		}
	}

	if relationshipFilter.RelationshipType != "" {
		finder = finder.Where("relationship_definition_dbs.type = ?", relationshipFilter.RelationshipType)
	}

	if relationshipFilter.SubType != "" {
		finder = finder.Where("relationship_definition_dbs.sub_type = ?", relationshipFilter.SubType)
	}
	if relationshipFilter.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", relationshipFilter.ModelName)
	}
	if relationshipFilter.Version != "" {
		finder = finder.Where("model_dbs.version = ?", relationshipFilter.Version)
	}
	if relationshipFilter.OrderOn != "" {
		if relationshipFilter.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: relationshipFilter.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(relationshipFilter.OrderOn)
		}
	}

	var count int64
	finder.Count(&count)

	finder = finder.Offset(relationshipFilter.Offset)
	if relationshipFilter.Limit != 0 {
		finder = finder.Limit(relationshipFilter.Limit)
	}
	err = finder.
		Scan(&relationshipDefinitionsWithModel).Error
	if err != nil {
		fmt.Println(err.Error()) //for debugging
	}
	defs := make([]RelationshipDefinition, len(relationshipDefinitionsWithModel))

	// remove this when reldef and reldefdb struct is consolidated.
	for _, cm := range relationshipDefinitionsWithModel {
		// Ensure correct reg is passed, rn it is dummy for sake of testing.
		// In the first query above where we do seelection i think there changes will be requrired, an when that two def and defDB structs are consolidated, using association and preload i think we can do.
		reg := registry.Hostv1beta1{}
		defs = append(defs, cm.RelationshipDefinitionDB.GetRelationshipDefinition(cm.ModelDB.GetModel(cm.CategoryDB.GetCategory(db), reg)))
	}
	// Should have count unique relationships (by model version, model name, and relationship's kind, type, subtype, version)
	return defs, count, int(count), nil
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

func (m *RelationshipDefinition) UpdateStatus(db database.Handler, status entity.EntityStatus) error {
	return nil
}

func (r *RelationshipDefinition) GetRelationshipDefinitionDB() (rdb RelationshipDefinitionDB) {
	// rdb.ID = r.ID id will be assigned by the database itself don't use this, as it will be always uuid.nil, because id is not known when comp gets generated.
	// While database creates an entry with valid primary key but to avoid confusion, it is disabled and accidental assignment of custom id.
	rdb.VersionMeta = r.VersionMeta
	rdb.Kind = r.Kind
	rdb.RelationshipType = r.RelationshipType
	rdb.SubType = r.SubType
	rdb.EvaluationQuery = r.EvaluationQuery
	rdb.Metadata, _ = json.Marshal(r.Metadata)
	rdb.ModelID = r.Model.ID
	rdb.Selectors, _ = json.Marshal(r.Selectors)
	return
}

func (c RelationshipDefinition) WriteComponentDefinition(relDirPath string) error {
	relPath := filepath.Join(relDirPath, c.Kind, string(c.Type())+".json")
	err := utils.WriteJSONToFile[RelationshipDefinition](relPath, c)
	return err
}

func (rdb *RelationshipDefinitionDB) GetRelationshipDefinition(m v1beta1.Model) (r RelationshipDefinition) {
	r.ID = rdb.ID
	r.VersionMeta = rdb.VersionMeta
	r.Kind = rdb.Kind
	r.RelationshipType = rdb.RelationshipType
	r.SubType = rdb.SubType
	r.EvaluationQuery = rdb.EvaluationQuery
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Metadata, &r.Metadata)
	r.Model = m
	if r.Selectors == nil {
		r.Selectors = []map[string]interface{}{}
	}
	_ = json.Unmarshal(rdb.Selectors, &r.Selectors)
	return
}
