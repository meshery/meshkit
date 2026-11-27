package database

import (
	"testing"
)

func TestBuildPostgresDSN(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected string
	}{
		{
			name: "Postgres options without sslmode",
			opts: Options{
				Host:     "localhost",
				Port:     "5432",
				Username: "testuser",
				Password: "secretpassword",
				Database: "meshery",
			},
			expected: "host=localhost user=testuser password=secretpassword port=5432 dbname=meshery",
		},
		{
			name: "Postgres options with sslmode disable",
			opts: Options{
				Host:     "postgres.internal",
				Port:     "5432",
				Username: "admin",
				Password: "password123",
				Database: "mesherydb",
				SSLMode:  "disable",
			},
			expected: "host=postgres.internal user=admin password=password123 port=5432 dbname=mesherydb sslmode=disable",
		},
		{
			name: "Postgres options with custom port and sslmode require",
			opts: Options{
				Host:     "10.0.0.12",
				Port:     "5433",
				Username: "clouduser",
				Password: "mypassword",
				Database: "production",
				SSLMode:  "require",
			},
			expected: "host=10.0.0.12 user=clouduser password=mypassword port=5433 dbname=production sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := buildPostgresDSN(tt.opts)
			if actual != tt.expected {
				t.Errorf("buildPostgresDSN() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestNew_Postgres_EmptyOrWhitespaceDatabase(t *testing.T) {
	testCases := []struct {
		name     string
		database string
	}{
		{
			name:     "empty database name",
			database: "",
		},
		{
			name:     "whitespace-only database name",
			database: "   ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{
				Engine:   POSTGRES,
				Host:     "localhost",
				Port:     "5432",
				Username: "user",
				Password: "password",
				Database: tc.database,
			}

			_, err := New(opts)
			if err == nil {
				t.Fatalf("expected error when Database is %q for Postgres engine, got nil", tc.database)
			}

			if err != ErrNoneDatabase {
				t.Errorf("expected ErrNoneDatabase, got: %v", err)
			}
		})
	}
}

func TestNew_SQLite_Success_And_Close(t *testing.T) {
	opts := Options{
		Engine:   SQLITE,
		Filename: ":memory:",
	}

	handler, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error creating in-memory SQLite database: %v", err)
	}

	if handler.DB == nil {
		t.Fatal("expected valid gorm.DB instance, got nil")
	}

	if handler.Mutex == nil {
		t.Fatal("expected valid Mutex instance, got nil")
	}

	if err := handler.DBClose(); err != nil {
		t.Fatalf("unexpected error closing database: %v", err)
	}
}
