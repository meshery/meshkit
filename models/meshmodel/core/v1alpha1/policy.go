package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
	"gorm.io/gorm"
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

type PolicyFilter struct {
	Kind    string
	SubType string
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
		Joins("JOIN models ON models.id = policy_definition_dbs.model_id")
	if f.Kind != "" {
		finder = finder.Where("policy_definition_dbs.kind = ?", f.Kind)
	}
	if f.SubType != "" {
		finder = finder.Where("policy_definition_dbs.sub_type = ?", f.SubType)
	}
	if f.ModelName != "" {
		finder = finder.Where("models.name = ?", f.ModelName)
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
	if p.Expression == nil {
		p.Expression = make(map[string]interface{})
	}
	_ = json.Unmarshal(pdb.Expression, &p.Expression)

	return
}

func CreatePolicy(db *database.Handler, p PolicyDefinition) (uuid.UUID, error) {
	p.ID = uuid.New()
	tempModelID := uuid.New()
	byt, err := json.Marshal(p.Model)
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
	if model.ID == tempModelID || err == gorm.ErrRecordNotFound {
		model = p.Model
		model.ID = modelID
		err = db.Create(&model).Error
		if err != nil {
			modelCreationLock.Unlock()
			return uuid.UUID{}, err
		}
	}
	modelCreationLock.Unlock()
	pdb := p.GetPolicyDefinitionDB()
	pdb.ModelID = model.ID
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
	pdb.Expression, _ = json.Marshal(p.Expression)
	return
}
