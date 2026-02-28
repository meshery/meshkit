package v1beta1

import (
	"path/filepath"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/stretchr/testify/require"
)

func TestComponentSummary_GetSummary(t *testing.T) {
	db := newComponentSummaryTestDB(t)
	seedComponentSummaryData(t, db)

	f := &ComponentSummaryFilter{
		Include: []ComponentSummaryDimension{
			ComponentSummaryByModel,
			ComponentSummaryByRegistrant,
		},
	}

	summary, err := f.GetSummary(db)
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Total)

	require.Equal(t, map[string]int{"model-a": 2, "model-b": 1}, componentGroupToMap(summary.ByModel))
	require.Equal(t, map[string]int{"registrant-a": 2, "registrant-b": 1}, componentGroupToMap(summary.ByRegistrant))
	require.Empty(t, summary.ByCategory)
}

func TestComponentSummary_Validate(t *testing.T) {
	f := &ComponentSummaryFilter{
		Include: []ComponentSummaryDimension{"unknown_dimension"},
	}
	err := f.Validate()
	require.Error(t, err)
}

func newComponentSummaryTestDB(t *testing.T) *database.Handler {
	t.Helper()

	h, err := database.New(database.Options{
		Engine:   database.SQLITE,
		Filename: filepath.Join(t.TempDir(), "component-summary.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = h.DBClose()
	})

	err = h.AutoMigrate(
		&connection.Connection{},
		&category.CategoryDefinition{},
		&model.ModelDefinition{},
		&component.ComponentDefinition{},
	)
	require.NoError(t, err)

	return &h
}

func seedComponentSummaryData(t *testing.T, db *database.Handler) {
	t.Helper()

	connA := connection.Connection{ID: uuid.Must(uuid.NewV4()), Name: "registrant-a"}
	connB := connection.Connection{ID: uuid.Must(uuid.NewV4()), Name: "registrant-b"}
	require.NoError(t, db.Create(&connA).Error)
	require.NoError(t, db.Create(&connB).Error)

	cat := category.CategoryDefinition{
		Id:   uuid.Must(uuid.NewV4()),
		Name: "infra",
	}
	require.NoError(t, db.Create(&cat).Error)

	modelStatus := model.ModelDefinitionStatus("enabled")
	modelA := model.ModelDefinition{
		Id:           uuid.Must(uuid.NewV4()),
		Name:         "model-a",
		DisplayName:  "Model A",
		Status:       modelStatus,
		CategoryId:   cat.Id,
		RegistrantId: connA.ID,
		Model: struct {
			Version string `json:"version" yaml:"version"`
		}{Version: "v1.0.0"},
	}
	modelB := model.ModelDefinition{
		Id:           uuid.Must(uuid.NewV4()),
		Name:         "model-b",
		DisplayName:  "Model B",
		Status:       modelStatus,
		CategoryId:   cat.Id,
		RegistrantId: connB.ID,
		Model: struct {
			Version string `json:"version" yaml:"version"`
		}{Version: "v1.0.0"},
	}
	require.NoError(t, db.Create(&modelA).Error)
	require.NoError(t, db.Create(&modelB).Error)

	componentStatus := component.ComponentDefinitionStatus("enabled")
	comp1 := component.ComponentDefinition{
		Id:          uuid.Must(uuid.NewV4()),
		DisplayName: "comp-1",
		Status:      &componentStatus,
		ModelId:     modelA.Id,
		Component: component.Component{
			Kind:    "Deployment",
			Version: "v1",
		},
	}
	comp2 := component.ComponentDefinition{
		Id:          uuid.Must(uuid.NewV4()),
		DisplayName: "comp-2",
		Status:      &componentStatus,
		ModelId:     modelA.Id,
		Component: component.Component{
			Kind:    "Service",
			Version: "v1",
		},
	}
	comp3 := component.ComponentDefinition{
		Id:          uuid.Must(uuid.NewV4()),
		DisplayName: "comp-3",
		Status:      &componentStatus,
		ModelId:     modelB.Id,
		Component: component.Component{
			Kind:    "Pod",
			Version: "v1",
		},
	}
	require.NoError(t, db.Create(&comp1).Error)
	require.NoError(t, db.Create(&comp2).Error)
	require.NoError(t, db.Create(&comp3).Error)
}

func componentGroupToMap(rows []ComponentGroupEntry) map[string]int {
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out
}
