package v1alpha2

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/utils"
	"gorm.io/gorm/clause"
)

const SchemaVersion = "core.meshery.io/v1alpha2"

type RelationshipDefinition struct {
	ID uuid.UUID `json:"id"`
	v1beta1.VersionMeta
	Kind string `json:"kind,omitempty" yaml:"kind"`
	// The property has been named RelationshipType instead of Type to avoid collision from Type() function, which enables support for dynamic type.
	// Though, the column name and the json representation is "type".
	RelationshipType string                   `json:"type" yaml:"type" gorm:"type"`
	SubType          string                   `json:"subType" yaml:"subType"`
	EvaluationQuery  string                   `json:"evaluationQuery" yaml:"evaluationQuery" gorm:"evaluationQuery"`
	Metadata         map[string]interface{}   `json:"metadata"  yaml:"metadata" gorm:"type:bytes;serializer:json"`
	ModelID          uuid.UUID                `json:"-" gorm:"index:idx_relationship_definition_dbs_model_id,column:model_id"`
	Model            v1beta1.Model            `json:"model" gorm:"foreignKey:ModelID;references:ID"`
	Selectors        []map[string]interface{} `json:"selectors"  yaml:"selectors" gorm:"type:bytes;serializer:json"`
}

func (r RelationshipDefinition) TableName() string {
	return "relationship_definition_dbs"
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
	r.ModelID = mid
	err = db.Omit(clause.Associations).Create(&r).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return r.ID, err
}

func (m *RelationshipDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	return nil
}

func (c RelationshipDefinition) WriteComponentDefinition(relDirPath string) error {
	relPath := filepath.Join(relDirPath, c.Kind, string(c.Type())+".json")
	err := utils.WriteJSONToFile[RelationshipDefinition](relPath, c)
	return err
}

func (r *RelationshipDefinition) GetDefaultEvaluationQuery() string {
	return fmt.Sprintf("%s_%s_relationship", strings.ToLower(r.Kind), strings.ToLower(r.SubType))
}
