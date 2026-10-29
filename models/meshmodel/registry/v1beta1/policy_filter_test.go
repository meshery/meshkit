package v1beta1

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	core "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestPolicyFilterGetPaginationAndOrdering(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE model_dbs (id text PRIMARY KEY, name text)").Error)
	require.NoError(t, db.Exec("CREATE TABLE policy_definitions (id text PRIMARY KEY, kind text, sub_type text, modelID text)").Error)

	modelID, err := uuid.NewV4()
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO model_dbs (id, name) VALUES (?, ?)", modelID.String(), "test-model").Error)
	for _, kind := range []string{"zeta", "alpha", "mu"} {
		policyID, err := uuid.NewV4()
		require.NoError(t, err)
		require.NoError(t, db.Exec("INSERT INTO policy_definitions (id, kind, sub_type, modelID) VALUES (?, ?, ?, ?)", policyID.String(), kind, "test", modelID.String()).Error)
	}

	policies, count, total, err := (&PolicyFilter{OrderOn: "kind", Sort: "desc", Limit: 1, Offset: 1}).Get(&db)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
	require.Equal(t, 3, total)
	require.Len(t, policies, 1)
	require.Equal(t, "mu", policies[0].(*core.PolicyDefinition).Kind)

	policies, count, total, err = (&PolicyFilter{OrderOn: "kind"}).Get(&db)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
	require.Equal(t, 3, total)
	require.Len(t, policies, 3)
	require.Equal(t, "alpha", policies[0].(*core.PolicyDefinition).Kind)
}
