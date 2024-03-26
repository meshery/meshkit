package v1beta1

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/utils"

	"github.com/layer5io/meshkit/models/meshmodel/entity"
)

type PolicyDefinition struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Model      Model                  `json:"model"`
	SubType    string                 `json:"subType" yaml:"subType"`
	Expression map[string]interface{} `json:"expression" yaml:"expression"`
	CreatedAt  time.Time              `json:"-"`
	UpdatedAt  time.Time              `json:"-"`
}

type PolicyDefinitionDB struct {
	ID      uuid.UUID `json:"-"`
	ModelID uuid.UUID `json:"-" gorm:"modelID"`
	TypeMeta
	SubType    string    `json:"subType" yaml:"subType"`
	Expression []byte    `json:"expression" yaml:"expression"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

func (p PolicyDefinition) GetID() uuid.UUID {
	return p.ID
}

func (p PolicyDefinition) Type() entity.EntityType {
	return entity.PolicyDefinition
}

func (p *PolicyDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, definition version: %s, name: %s, model: %s, version: %s", p.Type(), p.Version, p.Kind, p.Model.Name, p.Model.Version)
}

func (p *PolicyDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	p.ID = uuid.New()
	mid, err := p.Model.Create(db, hostID)
	if err != nil {
		return uuid.UUID{}, err
	}
	pdb := p.GetPolicyDefinitionDB()
	pdb.ModelID = mid
	err = db.Create(&pdb).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return pdb.ID, nil
}

func (m *PolicyDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	return nil
}

func (p *PolicyDefinition) GetPolicyDefinitionDB() (pdb PolicyDefinitionDB) {
	pdb.ID = p.ID
	pdb.TypeMeta = p.TypeMeta
	pdb.SubType = p.SubType
	pdb.ModelID = p.Model.ID
	pdb.Expression, _ = json.Marshal(p.Expression)
	return
}

func (pdb *PolicyDefinitionDB) GetPolicyDefinition(m Model) (p PolicyDefinition) {
	p.ID = pdb.ID
	p.TypeMeta = pdb.TypeMeta
	p.Model = m
	p.SubType = pdb.SubType
	if p.Expression == nil {
		p.Expression = make(map[string]interface{})
	}
	_ = json.Unmarshal(pdb.Expression, &p.Expression)

	return
}

func (p PolicyDefinition) WritePolicyDefinition(policyDirPath string) error {
	policyPath := filepath.Join(policyDirPath, p.Kind+".json")
	err := utils.WriteJSONToFile[PolicyDefinition](policyPath, p)
	return err
}
