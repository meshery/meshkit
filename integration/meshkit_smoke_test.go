package integration_test

import (
	"strings"
	"testing"

	mkerrors "github.com/meshery/meshkit/errors"
)

func TestMeshKitFunctionalFlow(t *testing.T) {
	err := mkerrors.New(
		"11000",
		mkerrors.Alert,
		[]string{"MeshKit smoke check"},
		[]string{"Functional pipeline validation"},
		[]string{"Unknown"},
		[]string{"Review CI logs"},
	)

	if code := mkerrors.GetCode(err); code != "11000" {
		t.Fatalf("expected error code 11000, got %q", code)
	}

	message := err.Error()
	if !strings.Contains(message, "Functional pipeline validation") {
		t.Fatalf("expected rendered message to include long description, got %q", message)
	}
}
