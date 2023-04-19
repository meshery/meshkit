package coder

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/layer5io/meshkit/errors"         //nolint
	. "github.com/layer5io/meshkit/errors"       //nolint
	mesherr "github.com/layer5io/meshkit/errors" //nolint
)

var ErrTestCode1 = "one" //nolint:unused
var newDefaultTestFile = "errorsnewdefault_test.go"

//nolint:unused
func ErrNewDefaultTestOne(err error) error {
	return errors.NewDefault(ErrTestCode1, "Connection to broker failed", err.Error()) //nolint:staticcheck
}

//nolint:unused
func ErrNewDefaultTestTwo(err error) error {
	return NewDefault(ErrTestCode1, "Connection to broker failed", err.Error()) //nolint:staticcheck
}

//nolint:unused
func ErrNewDefaultTestThree(err error) error {
	return mesherr.NewDefault(ErrTestCode1, "Connection to broker failed", err.Error()) //nolint:staticcheck
}

func TestDetectNewDefault(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), newDefaultTestFile, nil, parser.ParseComments)
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	pkgs := []string{}
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if pgk, ok := isNewDefaultCallExpr(n); ok {
			found++
			pkgs = append(pkgs, pgk)
			// No need to look at children, this might lead to duplicates detections in case of dot-import.
			return false
		}
		return true
	})
	if !stringSliceContains(pkgs, "errors") {
		t.Errorf("package id 'errors' expected but not found. pkgs = %v", pkgs)
	}
	if !stringSliceContains(pkgs, "mesherr") {
		t.Errorf("package id 'mesherr' expected but not found. pkgs = %v", pkgs)
	}
	if !stringSliceContains(pkgs, "") {
		t.Errorf("package id '' expected but not found. pkgs = %v", pkgs)
	}
	countExpected := 3
	if len(pkgs) != countExpected {
		t.Errorf("found %v package ids; want %d", len(pkgs), countExpected)
	}
	if found != countExpected {
		t.Errorf("found %v call expressions; want %d", found, countExpected)
	}
}
