package v1beta1

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
)

type PolicyFilter struct {
	Kind      string
	Greedy    bool
	SubType   string
	ModelName string
	OrderOn   string
	Sort      string
	Limit     int
	Offset    int
}

func (pf *PolicyFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

type policyDefinitionWithModel struct {
	v1beta1.PolicyDefinitionDB
	Model v1beta1.Model
}

func (pf *PolicyFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	pl := []entity.Entity{}
	var componentDefinitionsWithModel []policyDefinitionWithModel
	finder := db.Model(&v1beta1.PolicyDefinitionDB{}).
		Select("policy_definition_dbs.*, models_dbs.*").
		Joins("JOIN model_dbs ON model_dbs.id = policy_definition_dbs.model_id")
	if pf.Kind != "" {
		finder = finder.Where("policy_definition_dbs.kind = ?", pf.Kind)
	}
	if pf.SubType != "" {
		finder = finder.Where("policy_definition_dbs.sub_type = ?", pf.SubType)
	}
	if pf.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", pf.ModelName)
	}

	var count int64
	finder.Count(&count)

	err := finder.Scan(&componentDefinitionsWithModel).Error
	if err != nil {
		return pl, 0, 0, err
	}
	for _, cm := range componentDefinitionsWithModel {
		policyDef := cm.PolicyDefinitionDB.GetPolicyDefinition(cm.Model)
		pl = append(pl, &policyDef)
	}
	return pl, count, int(count), nil
}
