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

// PolicyFilter selects policy definitions and controls their ordering and pagination.
// When ModelName is set, only policies associated with a matching model are included;
// policies without an associated model are excluded. Without ModelName, no model join
// is applied.
type PolicyFilter struct {
	Id        string
	Kind      string
	Greedy    bool
	SubType   string
	ModelName string
	// OrderOn accepts id, kind, version, modelID, sub_type, expression, created_at,
	// or updated_at. An unsupported value falls back to id. An empty value leaves
	// results unordered unless pagination requires an id ordering.
	OrderOn string
	// Sort orders the OrderOn column descending only when set to "desc"; otherwise
	// that column is ascending. The pagination tie-breaker, when added, is ascending.
	Sort string
	// Limit is the maximum number of results. Zero means no limit.
	Limit int
	// Offset is the number of matching results to skip. Zero means no offset.
	Offset int
}

func (pf *PolicyFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

// GetById retrieves a policy definition by its ID. It returns the entity and an error if any occurred.
func (pf *PolicyFilter) GetById(db *database.Handler) (entity.Entity, error) {
	p := &v1beta1.PolicyDefinition{}
	err := db.First(p, "id = ?", pf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, pf.Id)
	}
	return p, err
}

// Get retrieves policy definitions matching pf. When ModelName is set, only
// policies associated with a model of that name are included; policies without an
// associated model are excluded. Ordering and pagination follow the PolicyFilter
// settings. When pagination is requested and the primary order is not id, results
// use ascending id as a tie-breaker.
//
// The int64 count and int total returns both report the number of matching policies
// before pagination, not the number of policies in the returned slice. A zero Limit
// is unlimited; a zero Offset skips no results. Negative pagination values return
// an error.
func (pf *PolicyFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	pl := []entity.Entity{}
	if pf.Limit < 0 || pf.Offset < 0 {
		return pl, 0, 0, registry.ErrInvalidPolicyPagination()
	}

	var policyDefinitionWithModel []v1beta1.PolicyDefinition

	finder := db.Model(&v1beta1.PolicyDefinition{}).
		Select("policy_definitions.*")
	if pf.Kind != "" {
		finder = finder.Where("policy_definitions.kind = ?", pf.Kind)
	}
	if pf.SubType != "" {
		finder = finder.Where("policy_definitions.sub_type = ?", pf.SubType)
	}
	if pf.ModelName != "" {
		finder = finder.Joins(policyDefinitionModelJoin).Where("model_dbs.name = ?", pf.ModelName)
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
		finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Table: "policy_definitions", Name: orderColumn}, Desc: pf.Sort == "desc"})
	}
	if (pf.Limit != 0 || pf.Offset != 0) && orderColumn != "id" {
		finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Table: "policy_definitions", Name: "id"}})
	}

	var count int64
	if err := finder.Session(&gorm.Session{}).Select("count(*)").Count(&count).Error; err != nil {
		return pl, 0, 0, err
	}

	if pf.Limit != 0 {
		finder = finder.Limit(pf.Limit)
	}
	if pf.Offset != 0 {
		finder = finder.Offset(pf.Offset)
	}

	err := finder.Scan(&policyDefinitionWithModel).Error
	if err != nil {
		return pl, 0, 0, err
	}
	for _, pm := range policyDefinitionWithModel {
		pl = append(pl, &pm)
	}
	return pl, count, int(count), nil
}
