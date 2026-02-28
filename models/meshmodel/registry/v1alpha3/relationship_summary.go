package v1alpha3

import (
	"fmt"
	"slices"

	"github.com/meshery/meshkit/database"
	"github.com/meshery/schemas/models/v1alpha3/relationship"

	"gorm.io/gorm"
)

type RelationshipSummaryFilter struct {
	Kind             string
	Greedy           bool
	SubType          string
	RelationshipType string
	Version          string
	ModelName        string
	Status           string
	Include          []RelationshipSummaryDimension
}

type RelationshipSummaryDimension string

const (
	RelationshipSummaryByModel   RelationshipSummaryDimension = "by_model"
	RelationshipSummaryByKind    RelationshipSummaryDimension = "by_kind"
	RelationshipSummaryByType    RelationshipSummaryDimension = "by_type"
	RelationshipSummaryBySubType RelationshipSummaryDimension = "by_subtype"
)

type RelationshipGroupEntry struct {
	Key   string
	Count int
}

func (r RelationshipGroupEntry) KeyValue() string {
	return r.Key
}
func (r RelationshipGroupEntry) CountValue() int {
	return r.Count
}

type RelationshipSummary struct {
	Total     int64
	ByModel   []RelationshipGroupEntry
	ByKind    []RelationshipGroupEntry
	ByType    []RelationshipGroupEntry
	BySubType []RelationshipGroupEntry
}

func (relationshipFilter *RelationshipSummaryFilter) Validate() error {
	for _, dim := range relationshipFilter.Include {
		switch dim {
		case RelationshipSummaryByModel, RelationshipSummaryByKind, RelationshipSummaryByType, RelationshipSummaryBySubType:
			// valid
		default:
			return fmt.Errorf("unknown include dimension %s", dim)
		}
	}
	return nil
}

func (relationshipFilter *RelationshipSummaryFilter) GetSummary(db *database.Handler) (*RelationshipSummary, error) {
	if err := relationshipFilter.Validate(); err != nil {
		return nil, err
	}
	summary := &RelationshipSummary{}

	base := db.Model(&relationship.RelationshipDefinition{}).
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")

	status := "enabled"

	if relationshipFilter.Status != "" {
		status = relationshipFilter.Status
	}

	base = base.Where("model_dbs.status = ?", status)

	if relationshipFilter.Kind != "" {
		if relationshipFilter.Greedy {
			base = base.Where("relationship_definition_dbs.kind LIKE ?", "%"+relationshipFilter.Kind+"%")
		} else {
			base = base.Where("relationship_definition_dbs.kind = ?", relationshipFilter.Kind)
		}
	}

	if relationshipFilter.RelationshipType != "" {
		base = base.Where("relationship_definition_dbs.type = ?", relationshipFilter.RelationshipType)
	}

	if relationshipFilter.SubType != "" {
		base = base.Where("relationship_definition_dbs.sub_type = ?", relationshipFilter.SubType)
	}
	if relationshipFilter.ModelName != "" {
		base = base.Where("model_dbs.name = ?", relationshipFilter.ModelName)
	}
	if relationshipFilter.Version != "" {
		base = base.Where("model_dbs.model->>'version' = ?", relationshipFilter.Version)
	}
	if err := base.Session(&gorm.Session{}).
		Distinct("relationship_definition_dbs.id").
		Count(&summary.Total).Error; err != nil {
		return nil, err
	}

	shouldCompute := func(dim RelationshipSummaryDimension) bool {
		if len(relationshipFilter.Include) == 0 {
			return true
		}

		return slices.Contains(relationshipFilter.Include, dim)
	}

	type dimensionInfo struct {
		dim        RelationshipSummaryDimension
		selectExpr string
		groupExpr  string
		receiver   *[]RelationshipGroupEntry
	}

	dimensions := []dimensionInfo{
		{RelationshipSummaryByModel, "model_dbs.name as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count", "model_dbs.name", &summary.ByModel},
		{RelationshipSummaryByKind, "relationship_definition_dbs.kind as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count", "relationship_definition_dbs.kind", &summary.ByKind},
		{RelationshipSummaryByType, "relationship_definition_dbs.type as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count", "relationship_definition_dbs.type", &summary.ByType},
		{RelationshipSummaryBySubType, "relationship_definition_dbs.sub_type as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count", "relationship_definition_dbs.sub_type", &summary.BySubType},
	}

	for _, d := range dimensions {
		if shouldCompute(d.dim) {
			var rows []RelationshipGroupEntry
			err := base.Session(&gorm.Session{}).
				Select(d.selectExpr).
				Group(d.groupExpr).
				Scan(&rows).Error
			if err != nil {
				return nil, err
			}
			*d.receiver = rows
		}
	}
	return summary, nil
}
