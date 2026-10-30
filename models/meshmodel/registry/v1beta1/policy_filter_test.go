package v1beta1

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	meshkiterrors "github.com/meshery/meshkit/errors"
	core "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/meshkit/models/meshmodel/registry"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestPolicyFilterGetPaginationAndOrdering(t *testing.T) {
	db := newPolicyFilterTestDB(t)

	modelID := uuid.Must(uuid.FromString("00000000-0000-0000-0000-000000000001"))
	require.NoError(t, db.Table("model_dbs").Create(map[string]interface{}{"id": modelID.String(), "name": "test-model"}).Error)
	for _, policy := range []struct{ id, kind string }{
		{"00000000-0000-0000-0000-000000000002", "zeta"},
		{"00000000-0000-0000-0000-000000000003", "mu"},
		{"00000000-0000-0000-0000-000000000004", "mu"},
		{"00000000-0000-0000-0000-000000000005", "alpha"},
	} {
		policyID := uuid.Must(uuid.FromString(policy.id))
		require.NoError(t, db.Table("policy_definitions").Create(map[string]interface{}{"id": policyID.String(), "kind": policy.kind, "sub_type": "test", "modelID": modelID.String()}).Error)
	}

	tests := []struct {
		name      string
		filter    PolicyFilter
		wantKinds []string
		wantIDs   []string
	}{
		{
			name:      "descending pagination breaks ties by ID",
			filter:    PolicyFilter{OrderOn: "kind", Sort: "desc", Limit: 2, Offset: 1},
			wantKinds: []string{"mu", "mu"},
			wantIDs:   []string{"00000000-0000-0000-0000-000000000003", "00000000-0000-0000-0000-000000000004"},
		},
		{
			name:      "ascending order without pagination",
			filter:    PolicyFilter{OrderOn: "kind"},
			wantKinds: []string{"alpha", "mu", "mu", "zeta"},
		},
		{
			name:    "pagination without requested order uses ID",
			filter:  PolicyFilter{Limit: 1},
			wantIDs: []string{"00000000-0000-0000-0000-000000000002"},
		},
		{
			name:    "invalid order uses ID instead of raw SQL",
			filter:  PolicyFilter{OrderOn: "kind; invalid", Limit: 1},
			wantIDs: []string{"00000000-0000-0000-0000-000000000002"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, count, total, err := (&tt.filter).Get(&db)
			require.NoError(t, err)
			require.Equal(t, int64(4), count)
			require.Equal(t, 4, total)
			wantLen := len(tt.wantKinds)
			if len(tt.wantIDs) > wantLen {
				wantLen = len(tt.wantIDs)
			}
			require.Len(t, policies, wantLen)
			for i, policy := range policies {
				definition := policy.(*core.PolicyDefinition)
				if tt.wantKinds != nil {
					require.Equal(t, tt.wantKinds[i], definition.Kind)
				}
				if tt.wantIDs != nil {
					require.Equal(t, tt.wantIDs[i], definition.ID.String())
				}
			}
		})
	}
}

func TestPolicyFilterGetIncludesPoliciesWithoutModelUnlessFiltered(t *testing.T) {
	db := newPolicyFilterTestDB(t)
	policyID := uuid.Must(uuid.FromString("00000000-0000-0000-0000-000000000010"))
	require.NoError(t, db.Table("policy_definitions").Create(map[string]interface{}{
		"id": policyID.String(), "kind": "orphan", "sub_type": "test", "modelID": uuid.Nil.String(),
	}).Error)

	policies, count, _, err := (&PolicyFilter{}).Get(&db)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.Len(t, policies, 1)

	policies, count, _, err = (&PolicyFilter{ModelName: "missing"}).Get(&db)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, policies)
}

func TestPolicyFilterGetWrapsDatabaseErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		failOnCount bool
	}{
		{name: "count", failOnCount: true},
		{name: "scan"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := newPolicyFilterTestDB(t)

			wantErr := errors.New(tt.name + " query failed")
			if tt.failOnCount {
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_policy_count", func(tx *gorm.DB) {
					if _, ok := tx.Statement.Dest.(*int64); ok {
						tx.AddError(wantErr)
					}
				}))
			} else {
				require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:fail_policy_scan", func(tx *gorm.DB) {
					tx.AddError(wantErr)
				}))
			}

			_, _, _, err := (&PolicyFilter{}).Get(&db)
			var meshkitErr *meshkiterrors.ErrorV2
			require.ErrorAs(t, err, &meshkitErr)
			require.Equal(t, registry.ErrGetPolicyDefinitionsCode, meshkitErr.Code)
			require.Contains(t, meshkitErr.Error(), wantErr.Error())
		})
	}
}

func TestPolicyFilterGetRejectsNegativePagination(t *testing.T) {
	for _, filter := range []PolicyFilter{{Limit: -1}, {Offset: -1}} {
		_, _, _, err := (&filter).Get(nil)
		var meshkitErr *meshkiterrors.ErrorV2
		require.ErrorAs(t, err, &meshkitErr)
		require.Equal(t, registry.ErrInvalidPolicyPaginationCode, meshkitErr.Code)
	}
}

func TestPolicyFilterGetBuildsPostgresQuery(t *testing.T) {
	var queryLog bytes.Buffer
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               gormlogger.New(log.New(&queryLog, "", 0), gormlogger.Config{LogLevel: gormlogger.Info}),
	})
	require.NoError(t, err)

	handler := database.Handler{DB: db}
	_, _, _, err = (&PolicyFilter{ModelName: "test-model", OrderOn: "kind", Limit: 1}).Get(&handler)
	var meshkitErr *meshkiterrors.ErrorV2
	require.ErrorAs(t, err, &meshkitErr) // DryRun can build the query but cannot scan database rows.
	require.Contains(t, queryLog.String(), `JOIN model_dbs ON model_dbs.id = policy_definitions."modelID"`)
	require.Contains(t, queryLog.String(), `ORDER BY "policy_definitions"."kind"`)
	require.Contains(t, queryLog.String(), `"policy_definitions"."id"`)
}

func newPolicyFilterTestDB(t *testing.T) database.Handler {
	t.Helper()
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	_, err = registry.NewRegistryManager(&db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.DBClose() })
	return db
}
