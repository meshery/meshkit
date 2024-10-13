package v1beta1

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
)

type PolicyFilter struct {
	Id        string
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

func (pf *PolicyFilter) GetById(db *database.Handler) (entity.Entity, error) {
	p := &v1beta1.PolicyDefinition{}
	err := db.First(p, "id = ?", pf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, pf.Id)
	}
	return p, err
}

func (pf *PolicyFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	pl := []entity.Entity{}
	var policyDefinitionWithModel []v1beta1.PolicyDefinition
	finder := db.Model(&v1beta1.PolicyDefinition{}).
		Select("policy_definition_dbs.*").
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
	if pf.Id != "" {
		finder = finder.Where("policy_definition_dbs.id = ?", pf.Id)
	}
	var count int64
	finder.Count(&count)

	err := finder.Scan(&policyDefinitionWithModel).Error
	if err != nil {
		return pl, 0, 0, err
	}
	for _, pm := range policyDefinitionWithModel {
		pl = append(pl, &pm)
	}
	return pl, count, int(count), nil
}
