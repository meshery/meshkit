package registry

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/model"
	connectionv1beta3 "github.com/meshery/schemas/models/v1beta3/connection"
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

	hostID := uuid.Must(uuid.NewV4())

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

func TestRegisterEntityDeduplicatesModelsAcrossRegistrants(t *testing.T) {
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

	registrantA := connectionv1beta3.Connection{
		Kind: "artifacthub",
		Name: "Artifact Hub",
	}
	registrantB := connectionv1beta3.Connection{
		Kind: "github",
		Name: "GitHub",
	}

	createModel := func() model.ModelDefinition {
		return model.ModelDefinition{
			SchemaVersion: v1beta1.ModelSchemaVersion,
			Version:       "1.0.0",
			Name:          "kubernetes",
			DisplayName:   "Kubernetes",
			Status:        model.Enabled,
			Category: category.CategoryDefinition{
				Name: "Orchestration",
			},
			Model: model.Model{
				Version: "v1.31.0",
			},
		}
	}

	m1 := createModel()
	isRegErr, isEntityErr, err := rm.RegisterEntity(registrantA, &m1)
	require.NoError(t, err)
	assert.False(t, isRegErr)
	assert.False(t, isEntityErr)

	m2 := createModel()
	isRegErr, isEntityErr, err = rm.RegisterEntity(registrantB, &m2)
	require.NoError(t, err)
	assert.False(t, isRegErr)
	assert.True(t, isEntityErr) // Flags duplicate model

	var modelCount int64
	err = db.Model(&model.ModelDefinition{}).Where("name = ?", "kubernetes").Count(&modelCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), modelCount, "Expected exactly 1 canonical model row in model_dbs")

	var registryCount int64
	err = db.Table("registries").Where("entity = ?", m1.ID).Count(&registryCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), registryCount, "Expected 2 entries in registries table pointing to the same canonical model")

	// Test repeated registration by the same registrant (idempotency check)
	m1Repeat := createModel()
	isRegErr, isEntityErr, err = rm.RegisterEntity(registrantA, &m1Repeat)
	require.NoError(t, err)
	assert.False(t, isRegErr)
	assert.True(t, isEntityErr)

	// Counts must stay identical: 1 model row, 2 registries entries
	err = db.Model(&model.ModelDefinition{}).Where("name = ?", "kubernetes").Count(&modelCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), modelCount, "Expected exactly 1 canonical model row in model_dbs")

	err = db.Table("registries").Where("entity = ?", m1.ID).Count(&registryCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), registryCount, "Expected 2 entries in registries table (no duplicate links)")
}
