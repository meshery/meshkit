package coder

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/broker/nats"
	errutilerr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	"github.com/layer5io/meshkit/errors"
)

var ErrTestNewCode1 = "one" //nolint:unused
var ErrTestNewCode2 = "two" //nolint:unused
var newTestFile = "errorsnew_test.go"

//nolint:unused
func ErrNewTestOne(err error) error {
	return errors.New(ErrTestNewCode1, errors.Fatal, []string{}, []string{}, []string{}, []string{}) //nolint:staticcheck
}

//nolint:unused
func ErrNewTestTwo(err error) error {
	return errors.New(ErrTestNewCode2, errors.None, []string{"line11"}, []string{"line21", "line22"}, []string{}, []string{"line41"}) //nolint:staticcheck
}

//nolint:unused
func ErrNewTestNoMatch(err error) (broker.Handler, error) {
	return nats.New(nats.Options{}) //nolint:staticcheck
}

func TestDetectNew(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), newTestFile, nil, parser.ParseComments)
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	var errors []*errutilerr.Error
	ast.Inspect(file, func(n ast.Node) bool {
		if error, ok := isNewCallExpr(n); ok {
			errors = append(errors, error)
			// No need to look at children
			return false
		}
		return true
	})
	countExpected := 2
	found := len(errors)
	if found != countExpected {
		t.Errorf("found %v call expressions; want %d", found, countExpected)
	}
	for _, e := range errors {
		if !(e.Name == "ErrTestNewCode1" || e.Name == "ErrTestNewCode2") {
			t.Errorf("invalid error name found: %s; want %s", e.Name, "ErrTestNewCode1 or ErrTestNewCode2")
		}
		if e.Name == "ErrTestNewCode1" {
			if !(e.Severity == "Fatal" &&
				len(e.ShortDescription) == 0 &&
				len(e.LongDescription) == 0 &&
				len(e.ProbableCause) == 0 &&
				len(e.SuggestedRemediation) == 0) {
				t.Errorf("invalid error found: %v", e)
			}
		}
		if e.Name == "ErrTestNewCode2" {
			if !(e.Severity == "None" &&
				e.ShortDescription == "line11" &&
				e.LongDescription == "line21\nline22" &&
				len(e.ProbableCause) == 0 &&
				e.SuggestedRemediation == "line41") {
				t.Errorf("invalid error found: %v", e)
			}
		}
	}
}
