package v1alpha3

import (
	"fmt"
	"slices"

	"github.com/meshery/meshkit/database"
	relationship "github.com/meshery/schemas/models/v1alpha3/relationship"

	"gorm.io/gorm"
)

func GetSummary(relationshipFilter *relationship.RelationshipSummaryFilter, db *database.Handler) (*relationship.RelationshipSummary, error) {
	if err := validate(relationshipFilter); err != nil {
		return nil, err
	}

	summary := &relationship.RelationshipSummary{}

	base := db.Table("relationship_definition_dbs").
		Joins("JOIN model_dbs ON relationship_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id")

	status := "enabled"

	if relationshipFilter.Status != nil {
		status = *relationshipFilter.Status
	}

	base = base.Where("model_dbs.status = ?", status)

	if relationshipFilter.Kind != nil {
		greedy := relationshipFilter.Greedy != nil && *relationshipFilter.Greedy
		if greedy {
			base = base.Where("relationship_definition_dbs.kind LIKE ?", "%"+*relationshipFilter.Kind+"%")
		} else {
			base = base.Where("relationship_definition_dbs.kind = ?", *relationshipFilter.Kind)
		}
	}

	if relationshipFilter.RelationshipType != nil {
		base = base.Where("relationship_definition_dbs.type = ?", *relationshipFilter.RelationshipType)
	}

	if relationshipFilter.SubType != nil {
		base = base.Where("relationship_definition_dbs.sub_type = ?", *relationshipFilter.SubType)
	}
	if relationshipFilter.ModelName != nil {
		base = base.Where("model_dbs.name = ?", *relationshipFilter.ModelName)
	}
	if relationshipFilter.Version != nil {
		base = base.Where("model_dbs.model->>'version' = ?", *relationshipFilter.Version)
	}
	if err := base.Session(&gorm.Session{}).
		Distinct("relationship_definition_dbs.id").
		Count(&summary.Total).Error; err != nil {
		return nil, err
	}

	shouldCompute := func(dim relationship.RelationshipSummaryFilterInclude) bool {
		if relationshipFilter.Include == nil || len(*relationshipFilter.Include) == 0 {
			return true
		}

		return slices.Contains(*relationshipFilter.Include, dim)
	}

	type groupEntry = struct {
		Count int32  `json:"count" yaml:"count"`
		Key   string `json:"key" yaml:"key"`
	}

	type dimensionInfo struct {
		dim        relationship.RelationshipSummaryFilterInclude
		selectExpr string
		groupExpr  string
		setRows    func([]groupEntry)
	}

	dimensions := []dimensionInfo{
		{
			dim:        relationship.ByModel,
			selectExpr: "model_dbs.name as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count",
			groupExpr:  "model_dbs.name",
			setRows: func(rows []groupEntry) {
				summary.ByModel = &rows
			},
		},
		{
			dim:        relationship.ByKind,
			selectExpr: "relationship_definition_dbs.kind as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count",
			groupExpr:  "relationship_definition_dbs.kind",
			setRows: func(rows []groupEntry) {
				summary.ByKind = &rows
			},
		},
		{
			dim:        relationship.ByType,
			selectExpr: "relationship_definition_dbs.type as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count",
			groupExpr:  "relationship_definition_dbs.type",
			setRows: func(rows []groupEntry) {
				summary.ByType = &rows
			},
		},
		{
			dim:        relationship.BySubtype,
			selectExpr: "relationship_definition_dbs.sub_type as Key, COUNT(DISTINCT(relationship_definition_dbs.id)) as Count",
			groupExpr:  "relationship_definition_dbs.sub_type",
			setRows: func(rows []groupEntry) {
				summary.BySubType = &rows
			},
		},
	}

	for _, d := range dimensions {
		if shouldCompute(d.dim) {
			var rows []groupEntry
			err := base.Session(&gorm.Session{}).
				Select(d.selectExpr).
				Group(d.groupExpr).
				Scan(&rows).Error
			if err != nil {
				return nil, err
			}
			d.setRows(rows)
		}
	}
	return summary, nil
}

func validate(relationshipFilter *relationship.RelationshipSummaryFilter) error {
	if relationshipFilter == nil {
		return fmt.Errorf("nil relationship summary filter")
	}

	if relationshipFilter.Include == nil {
		return nil
	}

	for _, dim := range *relationshipFilter.Include {
		if !dim.Valid() {
			return fmt.Errorf("unknown include dimension %s", dim)
		}
	}
	return nil
}
