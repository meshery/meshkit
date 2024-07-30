package v1beta1

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofrs/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/utils"
	v1beta1 "github.com/meshery/schemas/models/v1beta1/model"
	"gorm.io/gorm/clause"

	"github.com/layer5io/meshkit/models/meshmodel/entity"
)

// swagger:response PolicyDefinition
type PolicyDefinition struct {
	ID         uuid.UUID               `json:"-"`
	Kind       string                  `json:"kind,omitempty" yaml:"kind"`
	Version    string                  `json:"version,omitempty" yaml:"version"`
	ModelID    uuid.UUID               `json:"-" gorm:"column:modelID"`
	Model      v1beta1.ModelDefinition `json:"model"`
	SubType    string                  `json:"subType" yaml:"subType"`
	Expression map[string]interface{}  `json:"expression" yaml:"expression" gorm:"type:bytes;serializer:json"`
	CreatedAt  time.Time               `json:"-"`
	UpdatedAt  time.Time               `json:"-"`
}

func (p PolicyDefinition) GetID() uuid.UUID {
	return p.ID
}

func (p *PolicyDefinition) GenerateID() (uuid.UUID, error) {
	return uuid.NewV4()
}

func (p PolicyDefinition) Type() entity.EntityType {
	return entity.PolicyDefinition
}

func (p *PolicyDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, definition version: %s, name: %s, model: %s, version: %s", p.Type(), p.Version, p.Kind, p.Model.Name, p.Model.Version)
}

func (p *PolicyDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	p.ID, _ = p.GenerateID()

	mid, err := p.Model.Create(db, hostID)
	if err != nil {
		return uuid.UUID{}, err
	}
	p.ModelID = mid
	err = db.Omit(clause.Associations).Create(&p).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return p.ID, nil
}

func (m *PolicyDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	return nil
}

func (p PolicyDefinition) WritePolicyDefinition(policyDirPath string) error {
	policyPath := filepath.Join(policyDirPath, p.Kind+".json")
	err := utils.WriteJSONToFile[PolicyDefinition](policyPath, p)
	return err
}
