package registry

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateEntityStatusUpdatesModel(t *testing.T) {
	db, err := database.New(database.Options{
		Engine:   database.SQLITE,
		Filename: ":memory:",
	})
	require.NoError(t, err)

	rm, err := NewRegistryManager(&db)
	require.NoError(t, err)
	t.Cleanup(func() {
		rm.Cleanup()
		assert.NoError(t, db.DBClose())
	})

	hostID, err := uuid.NewV4()
	require.NoError(t, err)

	modelDef := model.ModelDefinition{
		SchemaVersion: v1beta1.ModelSchemaVersion,
		Version:       "1.0.0",
		Name:          "test-model",
		DisplayName:   "Test Model",
		Status:        model.Enabled,
		Category: category.CategoryDefinition{
			Name: "test-category",
		},
		Model: model.Model{
			Version: "1.0.0",
		},
	}

	modelID, err := modelDef.Create(&db, hostID)
	require.NoError(t, err)

	err = rm.UpdateEntityStatus(modelID.String(), string(entity.Ignored), "models")
	require.NoError(t, err)

	var updated model.ModelDefinition
	err = db.First(&updated, "id = ?", modelID).Error
	require.NoError(t, err)
	assert.Equal(t, model.Ignored, updated.Status)
}

func TestUpdateEntityStatusReturnsErrorForInvalidUUID(t *testing.T) {
	rm := &RegistryManager{}

	err := rm.UpdateEntityStatus("not-a-uuid", string(entity.Ignored), "models")

	require.Error(t, err)
}
