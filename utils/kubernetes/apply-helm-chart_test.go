package kubernetes

import (
	"errors"
	"os"
	"testing"
)

func TestWithHelmSQLConnectionStringRestoresExistingValue(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")

	err := withHelmSQLConnectionString("temporary-value", func() error {
		if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "temporary-value" {
			t.Fatalf("SQL connection string during initialization = %q, want %q", got, "temporary-value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withHelmSQLConnectionString() error = %v", err)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("restored SQL connection string = %q, want %q", got, "existing-value")
	}
}

func TestWithHelmSQLConnectionStringRestoresAfterInitializationError(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")
	wantErr := errors.New("initialization failed")

	err := withHelmSQLConnectionString("temporary-value", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("withHelmSQLConnectionString() error = %v, want %v", err, wantErr)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("restored SQL connection string = %q, want %q", got, "existing-value")
	}
}

func TestWithHelmSQLConnectionStringChecksSetenvError(t *testing.T) {
	called := false
	err := withHelmSQLConnectionString("invalid\x00value", func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("withHelmSQLConnectionString() error = nil, want invalid environment value error")
	}
	if called {
		t.Fatal("initializer was called after os.Setenv failed")
	}
}

func TestCreateHelmActionConfigDoesNotSetSQLConnectionStringForNonSQLDriver(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")

	_, err := (&Client{}).createHelmActionConfig(ApplyHelmChartConfig{
		HelmDriver:          Secret,
		SQLConnectionString: "unused-sensitive-value",
	}, nil)
	if err != nil {
		t.Fatalf("createHelmActionConfig() error = %v", err)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("SQL connection string = %q, want unchanged value %q", got, "existing-value")
	}
}
