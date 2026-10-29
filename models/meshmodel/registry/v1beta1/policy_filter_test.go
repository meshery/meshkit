package v1beta1

import (
	"errors"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	core "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestPolicyFilterGetPaginationAndOrdering(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE model_dbs (id text PRIMARY KEY, name text)").Error)
	require.NoError(t, db.Exec("CREATE TABLE policy_definitions (id text PRIMARY KEY, kind text, sub_type text, modelID text)").Error)

	modelID := uuid.Must(uuid.FromString("00000000-0000-0000-0000-000000000001"))
	require.NoError(t, db.Exec("INSERT INTO model_dbs (id, name) VALUES (?, ?)", modelID.String(), "test-model").Error)
	for _, policy := range []struct{ id, kind string }{
		{"00000000-0000-0000-0000-000000000002", "zeta"},
		{"00000000-0000-0000-0000-000000000003", "mu"},
		{"00000000-0000-0000-0000-000000000004", "mu"},
		{"00000000-0000-0000-0000-000000000005", "alpha"},
	} {
		policyID := uuid.Must(uuid.FromString(policy.id))
		require.NoError(t, db.Exec("INSERT INTO policy_definitions (id, kind, sub_type, modelID) VALUES (?, ?, ?, ?)", policyID.String(), policy.kind, "test", modelID.String()).Error)
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

func TestPolicyFilterGetReturnsCountError(t *testing.T) {
	db, err := database.New(database.Options{Engine: database.SQLITE, Filename: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE model_dbs (id text PRIMARY KEY, name text)").Error)
	require.NoError(t, db.Exec("CREATE TABLE policy_definitions (id text PRIMARY KEY, kind text, sub_type text, modelID text)").Error)

	wantErr := errors.New("count query failed")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_policy_count", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*int64); ok {
			tx.AddError(wantErr)
		}
	}))

	_, _, _, err = (&PolicyFilter{}).Get(&db)
	require.ErrorIs(t, err, wantErr)
}

func TestPolicyFilterGetQuotesModelIDForPostgres(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	var policies []core.PolicyDefinition
	query := db.Model(&core.PolicyDefinition{}).Select("policy_definitions.*").Joins(policyDefinitionModelJoin).Find(&policies)
	require.NoError(t, query.Error)
	require.Contains(t, query.Statement.SQL.String(), `policy_definitions."modelID"`)
}
