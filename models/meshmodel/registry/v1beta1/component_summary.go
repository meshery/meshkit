package v1beta1

import (
	"fmt"
	"slices"

	"github.com/meshery/meshkit/database"
	"github.com/meshery/schemas/models/v1beta1/component"
	"gorm.io/gorm"
)

func GetSummary(componentFilter *component.ComponentSummaryFilter, db *database.Handler) (*component.ComponentSummary, error) {
	if err := validate(componentFilter); err != nil {
		return nil, err
	}

	summary := &component.ComponentSummary{}
	base := db.Model(&component.ComponentDefinition{}).
		Joins("JOIN model_dbs ON component_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN connections ON connections.id = model_dbs.connection_id")
	componentStatus := "enabled"
	if componentFilter.Status != nil {
		componentStatus = *componentFilter.Status
	}
	base = base.Where("component_definition_dbs.status = ?", componentStatus)
	if componentFilter.Annotations != nil {
		switch *componentFilter.Annotations {
		case component.True:
			base = base.Where("component_definition_dbs.metadata->>'isAnnotation' = true")
		case component.False:
			base = base.Where("component_definition_dbs.metadata->>'isAnnotation' = false")
		}
	}

	if componentFilter.ModelName != nil && *componentFilter.ModelName != "all" {
		base = base.Where("model_dbs.name = ?", *componentFilter.ModelName)
	}

	if componentFilter.CategoryName != nil {
		base = base.Where("category_dbs.name = ?", *componentFilter.CategoryName)
	}
	if componentFilter.Version != nil {
		base = base.Where("model_dbs.model->>'version' = ?", *componentFilter.Version)
	}
	if componentFilter.Registrant != nil {
		base = base.Where("connections.name = ?", *componentFilter.Registrant)
	}

	if err := base.Session(&gorm.Session{}).
		Distinct("component_definition_dbs.id").
		Count(&summary.Total).Error; err != nil {
		return nil, err
	}

	type groupEntry = struct {
		Count int32  `json:"count" yaml:"count"`
		Key   string `json:"key" yaml:"key"`
	}

	shouldCompute := func(dim component.ComponentSummaryFilterInclude) bool {
		if componentFilter.Include == nil || len(*componentFilter.Include) == 0 {
			return true
		}
		return slices.Contains(*componentFilter.Include, dim)
	}

	type dimensionInfo struct {
		dim        component.ComponentSummaryFilterInclude
		selectExpr string
		groupExpr  string
		setRows    func([]groupEntry)
	}

	dimensions := []dimensionInfo{
		{
			dim:        component.ByModel,
			selectExpr: "model_dbs.name as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count",
			groupExpr:  "model_dbs.name",
			setRows: func(rows []groupEntry) {
				summary.ByModel = &rows
			},
		},
		{
			dim:        component.ByCategory,
			selectExpr: "category_dbs.name as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count",
			groupExpr:  "category_dbs.name",
			setRows: func(rows []groupEntry) {
				summary.ByCategory = &rows
			},
		},
		{
			dim:        component.ByRegistrant,
			selectExpr: "connections.name as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count",
			groupExpr:  "connections.name",
			setRows: func(rows []groupEntry) {
				summary.ByRegistrant = &rows
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

func validate(componentFilter *component.ComponentSummaryFilter) error {
	if componentFilter == nil {
		return fmt.Errorf("nil component summary filter")
	}

	if componentFilter.Annotations != nil && !componentFilter.Annotations.Valid() {
		return fmt.Errorf("unknown annotations value %s", *componentFilter.Annotations)
	}

	if componentFilter.Include == nil {
		return nil
	}

	for _, dim := range *componentFilter.Include {
		if !dim.Valid() {
			return fmt.Errorf("unknown include dimension %s", dim)
		}
	}
	return nil
}
