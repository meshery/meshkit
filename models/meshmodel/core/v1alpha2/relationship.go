package v1alpha2

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/utils"
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

func (r RelationshipDefinition) Type() entity.EntityType {
	return entity.RelationshipDefinition
}
func (r RelationshipDefinition) GetID() uuid.UUID {
	return r.ID
}

func (r *RelationshipDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, definition version: %s, kind: %s, model: %s, version: %s", r.Type(), r.Version, r.Kind, r.Model.Name, r.Model.Version)
}

func (r *RelationshipDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	r.ID = uuid.New()
	mid, err := r.Model.Create(db, hostID)
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

func (m *RelationshipDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
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
