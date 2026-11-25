package v1beta1

import (
	"bytes"
	"errors"
	"log"
	"testing"
	"time"

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

func TestPolicyFilterGet_GivenSeededPolicies_WhenOrderedAndPaginated_ThenReturnsExpectedPage(t *testing.T) {
	db := newPolicyFilterTestDB(t)

	modelID := uuid.Must(uuid.FromString("00000000-0000-0000-0000-000000000001"))
	seedPolicyModel(t, &db, modelID.String(), "test-model")
	for _, policy := range []struct{ id, kind string }{
		{"00000000-0000-0000-0000-000000000002", "zeta"},
		{"00000000-0000-0000-0000-000000000003", "mu"},
		{"00000000-0000-0000-0000-000000000004", "mu"},
		{"00000000-0000-0000-0000-000000000005", "alpha"},
	} {
		seedPolicy(t, &db, policy.id, policy.kind, "test", modelID.String())
	}

	tests := []struct {
		name      string
		filter    PolicyFilter
		wantKinds []string
		wantIDs   []string
	}{
		{
			name:      "given duplicate kinds when sorted desc with limit and offset then ties break by ID",
			filter:    PolicyFilter{OrderOn: "kind", Sort: "desc", Limit: 2, Offset: 1},
			wantKinds: []string{"mu", "mu"},
			wantIDs:   []string{"00000000-0000-0000-0000-000000000003", "00000000-0000-0000-0000-000000000004"},
		},
		{
			name:      "given seeded policies when ordered by kind without pagination then returns all ascending",
			filter:    PolicyFilter{OrderOn: "kind"},
			wantKinds: []string{"alpha", "mu", "mu", "zeta"},
		},
		{
			name:    "given a limit and no order when getting then orders by ID",
			filter:  PolicyFilter{Limit: 1},
			wantIDs: []string{"00000000-0000-0000-0000-000000000002"},
		},
		{
			name:    "given an offset and no limit when getting then returns remaining policies ordered by ID",
			filter:  PolicyFilter{OrderOn: "id", Offset: 2},
			wantIDs: []string{"00000000-0000-0000-0000-000000000004", "00000000-0000-0000-0000-000000000005"},
		},
		{
			name:    "given an invalid order column when getting then orders by ID instead of raw SQL",
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

func TestPolicyFilterGet_GivenOrphanPolicy_WhenFilteringByModelName_ThenPolicyIsExcluded(t *testing.T) {
	db := newPolicyFilterTestDB(t)
	seedPolicy(t, &db, "00000000-0000-0000-0000-000000000010", "orphan", "test", uuid.Nil.String())

	policies, count, _, err := (&PolicyFilter{}).Get(&db)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.Len(t, policies, 1)

	policies, count, _, err = (&PolicyFilter{ModelName: "missing"}).Get(&db)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, policies)
}

func TestPolicyFilterGet_GivenSupportedOrderColumn_WhenSortedDescending_ThenReturnsPoliciesInOrder(t *testing.T) {
	const (
		modelID1  = "00000000-0000-0000-0000-000000000011"
		modelID2  = "00000000-0000-0000-0000-000000000012"
		policyID1 = "00000000-0000-0000-0000-000000000021"
		policyID2 = "00000000-0000-0000-0000-000000000022"
	)
	db := newPolicyFilterTestDB(t)
	older := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	for _, policy := range []map[string]interface{}{
		{
			"id": policyID1, "kind": "alpha", "version": "v1", "modelID": modelID1,
			"sub_type": "a", "expression": []byte(`{"rank":"a"}`), "created_at": older, "updated_at": older,
		},
		{
			"id": policyID2, "kind": "zeta", "version": "v2", "modelID": modelID2,
			"sub_type": "z", "expression": []byte(`{"rank":"z"}`), "created_at": newer, "updated_at": newer,
		},
	} {
		require.NoError(t, db.Table("policy_definitions").Create(policy).Error)
	}

	for _, orderOn := range []string{"id", "kind", "version", "modelID", "sub_type", "expression", "created_at", "updated_at"} {
		t.Run(orderOn, func(t *testing.T) {
			policies, _, _, err := (&PolicyFilter{OrderOn: orderOn, Sort: "desc"}).Get(&db)
			require.NoError(t, err)
			require.Len(t, policies, 2)
			require.Equal(t, policyID2, policies[0].(*core.PolicyDefinition).ID.String())
			require.Equal(t, policyID1, policies[1].(*core.PolicyDefinition).ID.String())
		})
	}
}

func TestPolicyFilterGet_GivenSeededPolicies_WhenFiltered_ThenReturnsMatchingPolicies(t *testing.T) {
	const (
		m1 = "00000000-0000-0000-0000-000000000101"
		m2 = "00000000-0000-0000-0000-000000000102"
		p1 = "00000000-0000-0000-0000-000000000201"
		p2 = "00000000-0000-0000-0000-000000000202"
		p3 = "00000000-0000-0000-0000-000000000203"
	)
	db := newPolicyFilterTestDB(t)
	seedPolicyModel(t, &db, m1, "model-one")
	seedPolicyModel(t, &db, m2, "model-two")
	seedPolicy(t, &db, p1, "a", "x", m1)
	seedPolicy(t, &db, p2, "b", "y", m1)
	seedPolicy(t, &db, p3, "a", "y", m2)

	tests := []struct {
		name    string
		filter  PolicyFilter
		wantIDs []string
	}{
		{"given policies when no filter then returns all", PolicyFilter{OrderOn: "id"}, []string{p1, p2, p3}},
		{"given policies when filtered by kind then returns matching kind", PolicyFilter{Kind: "a", OrderOn: "id"}, []string{p1, p3}},
		{"given policies when filtered by sub type then returns matching sub type", PolicyFilter{SubType: "y", OrderOn: "id"}, []string{p2, p3}},
		{"given policies when filtered by model name then returns that model's policies", PolicyFilter{ModelName: "model-one", OrderOn: "id"}, []string{p1, p2}},
		{"given policies when filtered by id then returns that policy", PolicyFilter{Id: p2}, []string{p2}},
		{"given policies when filtered by kind and model name then returns the intersection", PolicyFilter{Kind: "a", ModelName: "model-two"}, []string{p3}},
		{"given policies when no policy matches then returns empty", PolicyFilter{Kind: "missing"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, count, total, err := (&tt.filter).Get(&db)
			require.NoError(t, err)
			require.Equal(t, int64(len(tt.wantIDs)), count)
			require.Equal(t, len(tt.wantIDs), total)
			gotIDs := make([]string, 0, len(policies))
			for _, policy := range policies {
				gotIDs = append(gotIDs, policy.(*core.PolicyDefinition).ID.String())
			}
			require.ElementsMatch(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestPolicyFilterGet_GivenFailingQuery_WhenGetting_ThenReturnsDatabaseError(t *testing.T) {
	for _, tt := range []struct {
		name        string
		failOnCount bool
	}{
		{name: "given count query fails", failOnCount: true},
		{name: "given scan query fails"},
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
			require.ErrorIs(t, err, wantErr)
		})
	}
}

func TestPolicyFilterGet_GivenNegativeLimitOrOffset_WhenGetting_ThenReturnsInvalidPaginationError(t *testing.T) {
	for _, filter := range []PolicyFilter{{Limit: -1}, {Offset: -1}} {
		_, _, _, err := (&filter).Get(nil)
		var meshkitErr *meshkiterrors.Error
		require.ErrorAs(t, err, &meshkitErr)
		require.Equal(t, registry.ErrInvalidPolicyPaginationCode, meshkitErr.Code)
	}
}

func TestPolicyFilterGet_GivenPostgresDialect_WhenGetting_ThenBuildsExpectedQuery(t *testing.T) {
	var queryLog bytes.Buffer
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               gormlogger.New(log.New(&queryLog, "", 0), gormlogger.Config{LogLevel: gormlogger.Info}),
	})
	require.NoError(t, err)

	handler := database.Handler{DB: db}
	_, _, _, err = (&PolicyFilter{ModelName: "test-model", OrderOn: "kind", Limit: 1}).Get(&handler)
	require.ErrorContains(t, err, "dry run mode unsupported") // DryRun can build the query but cannot scan database rows.
	require.Contains(t, queryLog.String(), `JOIN model_dbs ON model_dbs.id = policy_definitions."modelID"`)
	require.Contains(t, queryLog.String(), `ORDER BY "policy_definitions"."kind","policy_definitions"."id"`)
	require.Contains(t, queryLog.String(), `LIMIT 1`)
}

func seedPolicyModel(t *testing.T, db *database.Handler, id, name string) {
	t.Helper()
	require.NoError(t, db.Table("model_dbs").Create(map[string]interface{}{"id": id, "name": name}).Error)
}

func seedPolicy(t *testing.T, db *database.Handler, id, kind, subType, modelID string) {
	t.Helper()
	require.NoError(t, db.Table("policy_definitions").Create(map[string]interface{}{
		"id": id, "kind": kind, "sub_type": subType, "modelID": modelID,
	}).Error)
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
