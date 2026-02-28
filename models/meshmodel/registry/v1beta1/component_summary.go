package v1beta1

import (
	"fmt"
	"slices"

	"github.com/meshery/meshkit/database"
	"github.com/meshery/schemas/models/v1beta1/component"
	"gorm.io/gorm"
)

type ComponentSummaryFilter struct {
	ModelName    string
	CategoryName string
	Version      string
	Status       string
	Annotations  string
	Registrant   string

	Include []ComponentSummaryDimension
}

type ComponentSummaryDimension string

const (
	ComponentSummaryByModel      ComponentSummaryDimension = "by_model"
	ComponentSummaryByCategory   ComponentSummaryDimension = "by_category"
	ComponentSummaryByRegistrant ComponentSummaryDimension = "by_registrant"
)

type ComponentGroupEntry struct {
	Key   string
	Count int
}

func (c ComponentGroupEntry) KeyValue() string {
	return c.Key
}
func (c ComponentGroupEntry) CountValue() int {
	return c.Count
}

type ComponentSummary struct {
	Total        int64
	ByModel      []ComponentGroupEntry
	ByCategory   []ComponentGroupEntry
	ByRegistrant []ComponentGroupEntry
}

func (componentSummaryFilter *ComponentSummaryFilter) Validate() error {
	for _, dim := range componentSummaryFilter.Include {
		switch dim {
		case ComponentSummaryByModel, ComponentSummaryByCategory, ComponentSummaryByRegistrant:
			// valid
		default:
			return fmt.Errorf("unknown include dimension %s", dim)
		}
	}
	return nil
}
func (componentFilter *ComponentSummaryFilter) GetSummary(db *database.Handler) (*ComponentSummary, error) {
	if err := componentFilter.Validate(); err != nil {
		return nil, err
	}
	summary := &ComponentSummary{}
	base := db.Model(&component.ComponentDefinition{}).
		Joins("JOIN model_dbs ON component_definition_dbs.model_id = model_dbs.id").
		Joins("JOIN category_dbs ON model_dbs.category_id = category_dbs.id").
		Joins("JOIN connections ON connections.id = model_dbs.connection_id")
	componentStatus := "enabled"
	if componentFilter.Status != "" {
		componentStatus = componentFilter.Status
	}
	base = base.Where("component_definition_dbs.status = ?", componentStatus)
	switch componentFilter.Annotations {
	case "true":
		base = base.Where("component_definition_dbs.metadata->>'isAnnotation' = true")
	case "false":
		base = base.Where("component_definition_dbs.metadata->>'isAnnotation' = false")
	}

	if componentFilter.ModelName != "" && componentFilter.ModelName != "all" {
		base = base.Where("model_dbs.name = ?", componentFilter.ModelName)
	}

	if componentFilter.CategoryName != "" {
		base = base.Where("category_dbs.name = ?", componentFilter.CategoryName)
	}
	if componentFilter.Version != "" {
		base = base.Where("model_dbs.model->>'version' = ?", componentFilter.Version)
	}
	if componentFilter.Registrant != "" {
		base = base.Where("connections.name = ?", componentFilter.Registrant)
	}

	if err := base.Session(&gorm.Session{}).
		Distinct("component_definition_dbs.id").
		Count(&summary.Total).Error; err != nil {
		return nil, err
	}
	// per dimension
	shouldCompute := func(dim ComponentSummaryDimension) bool {
		// compute all if no include dimension
		if len(componentFilter.Include) == 0 {
			return true
		}
		return slices.Contains(componentFilter.Include, dim)
	}
	type dimensionInfo struct {
		dim        ComponentSummaryDimension
		selectExpr string
		groupExpr  string
		receiver   *[]ComponentGroupEntry
	}

	dimensions := []dimensionInfo{
		{ComponentSummaryByModel, "model_dbs.name as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count", "model_dbs.name", &summary.ByModel},
		{ComponentSummaryByCategory, "model_dbs.category_id as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count", "model_dbs.category_id", &summary.ByCategory},
		{ComponentSummaryByRegistrant, "connections.name as Key, COUNT(DISTINCT(component_definition_dbs.id)) as Count", "connections.name", &summary.ByRegistrant},
	}

	// partial error is not tolerated so the populated summary should all be correct
	for _, d := range dimensions {
		if shouldCompute(d.dim) {
			var rows []ComponentGroupEntry
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
