package v1alpha3

import (
	"path/filepath"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/stretchr/testify/require"
)

type relationshipSummaryRecord struct {
	Id               uuid.UUID                                  `gorm:"column:id;primaryKey"`
	Kind             string                                     `gorm:"column:kind"`
	RelationshipType string                                     `gorm:"column:type"`
	SubType          string                                     `gorm:"column:sub_type"`
	Status           *relationship.RelationshipDefinitionStatus `gorm:"column:status"`
	ModelId          uuid.UUID                                  `gorm:"column:model_id"`
	Version          string                                     `gorm:"column:version"`
}

func (relationshipSummaryRecord) TableName() string {
	return "relationship_definition_dbs"
}

func TestRelationshipSummary_GetSummary(t *testing.T) {
	db := newRelationshipSummaryTestDB(t)
	seedRelationshipSummaryData(t, db)

	include := []relationship.RelationshipSummaryFilterInclude{relationship.ByModel, relationship.ByKind}
	f := &relationship.RelationshipSummaryFilter{Include: &include}

	summary, err := GetSummary(f, db)
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.Total)
	require.Equal(t, map[string]int32{"model-a": 1, "model-b": 1}, relationshipGroupToMap(summary.ByModel))
	require.Equal(t, map[string]int32{"edge": 1, "hierarchy": 1}, relationshipGroupToMap(summary.ByKind))
	require.Empty(t, summary.ByType)
	require.Empty(t, summary.BySubType)
}

func TestRelationshipSummary_Validate(t *testing.T) {
	include := []relationship.RelationshipSummaryFilterInclude{relationship.RelationshipSummaryFilterInclude("invalid_dimension")}
	f := &relationship.RelationshipSummaryFilter{Include: &include}
	_, err := GetSummary(f, newRelationshipSummaryTestDB(t))
	require.Error(t, err)
}

func newRelationshipSummaryTestDB(t *testing.T) *database.Handler {
	t.Helper()

	h, err := database.New(database.Options{
		Engine:   database.SQLITE,
		Filename: filepath.Join(t.TempDir(), "relationship-summary.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = h.DBClose()
	})

	err = h.AutoMigrate(
		&category.CategoryDefinition{},
		&model.ModelDefinition{},
		&relationshipSummaryRecord{},
	)
	require.NoError(t, err)

	return &h
}

func seedRelationshipSummaryData(t *testing.T, db *database.Handler) {
	t.Helper()

	cat := category.CategoryDefinition{
		Id:   uuid.Must(uuid.NewV4()),
		Name: "infra",
	}
	require.NoError(t, db.Create(&cat).Error)

	modelStatus := model.ModelDefinitionStatus("enabled")
	modelA := model.ModelDefinition{
		Id:          uuid.Must(uuid.NewV4()),
		Name:        "model-a",
		DisplayName: "Model A",
		Status:      modelStatus,
		CategoryId:  cat.Id,
		Model: struct {
			Version string `json:"version" yaml:"version"`
		}{Version: "v1.0.0"},
	}
	modelB := model.ModelDefinition{
		Id:          uuid.Must(uuid.NewV4()),
		Name:        "model-b",
		DisplayName: "Model B",
		Status:      modelStatus,
		CategoryId:  cat.Id,
		Model: struct {
			Version string `json:"version" yaml:"version"`
		}{Version: "v1.0.0"},
	}
	require.NoError(t, db.Create(&modelA).Error)
	require.NoError(t, db.Create(&modelB).Error)

	relationshipStatus := relationship.RelationshipDefinitionStatus("enabled")
	rel1 := relationshipSummaryRecord{
		Id:               uuid.Must(uuid.NewV4()),
		Kind:             "edge",
		RelationshipType: "binding",
		SubType:          "parent",
		Status:           &relationshipStatus,
		ModelId:          modelA.Id,
		Version:          "v1.0.0",
	}
	rel2 := relationshipSummaryRecord{
		Id:               uuid.Must(uuid.NewV4()),
		Kind:             "hierarchy",
		RelationshipType: "binding",
		SubType:          "child",
		Status:           &relationshipStatus,
		ModelId:          modelB.Id,
		Version:          "v1.0.0",
	}
	require.NoError(t, db.Create(&rel1).Error)
	require.NoError(t, db.Create(&rel2).Error)
}

func relationshipGroupToMap(rows *[]struct {
	Count int32  `json:"count" yaml:"count"`
	Key   string `json:"key" yaml:"key"`
}) map[string]int32 {
	if rows == nil {
		return map[string]int32{}
	}
	out := make(map[string]int32, len(*rows))
	for _, row := range *rows {
		out[row.Key] = row.Count
	}
	return out
}
