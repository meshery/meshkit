package v1beta1

import (
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/models/meshmodel/registry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const policyDefinitionModelJoin = `JOIN model_dbs ON model_dbs.id = policy_definitions."modelID"`

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
		Select("policy_definitions.*").
		Joins(policyDefinitionModelJoin)
	if pf.Kind != "" {
		finder = finder.Where("policy_definitions.kind = ?", pf.Kind)
	}
	if pf.SubType != "" {
		finder = finder.Where("policy_definitions.sub_type = ?", pf.SubType)
	}
	if pf.ModelName != "" {
		finder = finder.Where("model_dbs.name = ?", pf.ModelName)
	}
	if pf.Id != "" {
		finder = finder.Where("policy_definitions.id = ?", pf.Id)
	}
	orderColumn := ""
	switch pf.OrderOn {
	case "id", "kind", "version", "modelID", "sub_type", "expression", "created_at", "updated_at":
		orderColumn = pf.OrderOn
	case "":
	default:
		orderColumn = "id"
	}
	if orderColumn != "" {
		finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: orderColumn}, Desc: pf.Sort == "desc"})
	}
	if (pf.Limit != 0 || pf.Offset != 0) && orderColumn != "id" {
		finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}})
	}

	var count int64
	if err := finder.Session(&gorm.Session{}).Select("count(*)").Count(&count).Error; err != nil {
		return pl, 0, 0, registry.ErrGetPolicyDefinitions(err)
	}

	if pf.Limit != 0 {
		finder = finder.Limit(pf.Limit)
	}
	if pf.Offset != 0 {
		finder = finder.Offset(pf.Offset)
	}

	err := finder.Scan(&policyDefinitionWithModel).Error
	if err != nil {
		return pl, 0, 0, registry.ErrGetPolicyDefinitions(err)
	}
	for _, pm := range policyDefinitionWithModel {
		pl = append(pl, &pm)
	}
	return pl, count, int(count), nil
}
