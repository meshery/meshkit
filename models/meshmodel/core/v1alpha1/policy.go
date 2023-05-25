package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
)

type PolicyDefinition struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Model      Model                  `json:"model"`
	SubType    string                 `json:"subType" yaml:"subType"`
	Expression string `json:"expression" yaml:"expression"`
	Metadata  map[string]interface{} `json:"metadata" yaml:"metadata"`
	CreatedAt  time.Time              `json:"-"`
	UpdatedAt  time.Time              `json:"-"`
}

type PolicyDefinitionDB struct {
	ID      uuid.UUID `json:"-"`
	ModelID uuid.UUID `json:"-" gorm:"modelID"`
	TypeMeta
	SubType    string    `json:"subType" yaml:"subType"`
	Expression string    `json:"expression" yaml:"expression"`
	Metadata  []byte    `json:"metadata" yaml:"metadata"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

type PolicyFilter struct {
	Kind      string
	SubType   string
	ModelName string
}

func (pf *PolicyFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

func (p PolicyDefinition) GetID() uuid.UUID {
	return p.ID
}

func (p PolicyDefinition) Type() types.CapabilityType {
	return types.PolicyDefinition
}

func GetMeshModelPolicy(db *database.Handler, f PolicyFilter) (pl []PolicyDefinition) {
	type componentDefinitionWithModel struct {
		PolicyDefinitionDB
		Model
	}
	var componentDefinitionsWithModel []componentDefinitionWithModel
	finder := db.Model(&PolicyDefinitionDB{}).
		Select("policy_definition_dbs.*, models.*").
		Joins("JOIN model_dbs ON model_dbs.id = policy_definition_dbs.model_id")
	if f.Kind != "" {
		finder = finder.Where("policy_definition_dbs.kind = ?", f.Kind)
	}
	if f.SubType != "" {
		finder = finder.Where("policy_definition_dbs.sub_type = ?", f.SubType)
	}
	if f.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", f.ModelName)
	}
	err := finder.Scan(&componentDefinitionsWithModel).Error
	if err != nil {
		fmt.Println(err.Error())
	}
	for _, cm := range componentDefinitionsWithModel {
		pl = append(pl, cm.PolicyDefinitionDB.GetPolicyDefinition(cm.Model))
	}
	return pl
}

func (pdb *PolicyDefinitionDB) GetPolicyDefinition(m Model) (p PolicyDefinition) {
	p.ID = pdb.ID
	p.TypeMeta = pdb.TypeMeta
	p.Model = m
	p.SubType = pdb.SubType
	p.Expression = pdb.Expression
	if p.Metadata == nil {
		p.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(pdb.Metadata, &p.Metadata)

	return
}

func CreatePolicy(db *database.Handler, p PolicyDefinition) (uuid.UUID, error) {
	p.ID = uuid.New()
	mid, err := CreateModel(db, p.Model)
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

func (p *PolicyDefinition) GetPolicyDefinitionDB() (pdb PolicyDefinitionDB) {
	pdb.ID = p.ID
	pdb.TypeMeta = p.TypeMeta
	pdb.SubType = p.SubType
	pdb.ModelID = p.Model.ID
	pdb.Expression = p.Expression
	pdb.Metadata, _ = json.Marshal(p.Metadata)
	return
}
