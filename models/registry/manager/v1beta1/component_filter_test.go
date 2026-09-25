package v1beta1

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/registry/entity"
	"github.com/meshery/schemas/models/core"
	"github.com/meshery/schemas/models/v1beta3/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The four-table `SELECT *` scan in Get() drops the serializer-backed `styles`
// column: GORM cannot apply the json serializer through a multi-embedded
// composite scan, so components come back with Styles == nil even though the
// column holds valid JSON. hydrateComponentStyles must restore it from the
// authoritative single-table row.
func TestHydrateComponentStylesRestoresDroppedStyles(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&component.ComponentDefinition{}))

	wantSvg := map[string]string{}
	var stored []component.ComponentDefinition
	for _, kind := range []string{"BGPPolicy", "Egress", "Tier"} {
		id, err := uuid.NewV4()
		require.NoError(t, err)
		svg := "ui/public/static/img/meshmodels/antrea/color/" + kind + "-color.svg"
		cd := component.ComponentDefinition{
			ID:        id,
			Component: component.Component{Kind: kind, Version: "v1"},
			Styles:    &core.ComponentStyles{PrimaryColor: "#00c1d5", SvgColor: svg},
		}
		require.NoError(t, db.Create(&cd).Error)
		stored = append(stored, cd)
		wantSvg[id.String()] = svg
	}

	// Simulate the join scan dropping Styles on every returned component.
	defs := make([]entity.Entity, 0, len(stored))
	for i := range stored {
		dropped := stored[i]
		dropped.Styles = nil
		defs = append(defs, &dropped)
	}

	require.NoError(t, hydrateComponentStyles(&db, defs))

	for _, e := range defs {
		cd := e.(*component.ComponentDefinition)
		require.NotNilf(t, cd.Styles, "styles not re-hydrated for %s", cd.Component.Kind)
		assert.Equal(t, "#00c1d5", cd.Styles.PrimaryColor)
		assert.Equal(t, wantSvg[cd.ID.String()], cd.Styles.SvgColor)
	}
}

// Duplicate component rows in the input (the join can return duplicates) must be
// de-duplicated and still hydrated correctly.
func TestHydrateComponentStylesDeduplicatesIDs(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&component.ComponentDefinition{}))

	id, err := uuid.NewV4()
	require.NoError(t, err)
	require.NoError(t, db.Create(&component.ComponentDefinition{
		ID:        id,
		Component: component.Component{Kind: "Egress", Version: "v1"},
		Styles:    &core.ComponentStyles{PrimaryColor: "#00c1d5", SvgColor: "egress-color.svg"},
	}).Error)

	first := &component.ComponentDefinition{ID: id, Component: component.Component{Kind: "Egress", Version: "v1"}}
	second := &component.ComponentDefinition{ID: id, Component: component.Component{Kind: "Egress", Version: "v1"}}
	require.NoError(t, hydrateComponentStyles(&db, []entity.Entity{first, second}))

	for _, cd := range []*component.ComponentDefinition{first, second} {
		require.NotNil(t, cd.Styles)
		assert.Equal(t, "egress-color.svg", cd.Styles.SvgColor)
	}
}

// hydrateComponentStyles must be a no-op for components that already have styles
// and must tolerate an empty input slice.
func TestHydrateComponentStylesNoopWhenPopulated(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&component.ComponentDefinition{}))

	require.NoError(t, hydrateComponentStyles(&db, nil)) // must not error on empty input

	existing := &core.ComponentStyles{PrimaryColor: "#123456", SvgColor: "keep.svg"}
	cd := &component.ComponentDefinition{
		Component: component.Component{Kind: "Keep", Version: "v1"},
		Styles:    existing,
	}
	require.NoError(t, hydrateComponentStyles(&db, []entity.Entity{cd}))
	assert.Same(t, existing, cd.Styles, "already-populated styles must be left untouched")
}
