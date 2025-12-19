package coder

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/broker/nats"
	errutilerr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
	mesherr "github.com/meshery/meshkit/errors"
)

var ErrTestNewCode1 = "one"   //nolint:unused
var ErrTestNewCode2 = "two"   //nolint:unused
var ErrTestNewCode3 = "three" //nolint:unused
var newTestFile = "errorsnew_test.go"

//nolint:unused
func ErrNewTestOne(err error) error {
	return mesherr.New(ErrTestNewCode1, mesherr.Fatal, []string{}, []string{}, []string{}, []string{}) //nolint:staticcheck
}

//nolint:unused
func ErrNewTestTwo(err error) error {
	return mesherr.New(ErrTestNewCode2, mesherr.None, []string{"line11"}, []string{"line21", "line22"}, []string{}, []string{"line41"}) //nolint:staticcheck
}

//nolint:unused
func ErrNewTestThree(err error) error {
	// Note that all strings need to be literals, not call expressions. If a line contains a call expression, it is returned as empty string.
	// If the originating error should be propagated (which makes sense), it should be passed to New.Error(...) as separate parameter.
	// This would make for a cleaner contract, and allow the error to be handled separately, i.e. logged.
	// This is a consequence of how this error util tool extracts details statically.
	// The equivalent code with a new parameter could look like this:
	//	return mesherr.New(ErrTestNewCode3, mesherr.None, []string{"This error: %v"}, []string{"line21", "line22"}, []string{}, []string{"line41"}, err)
	return mesherr.New(ErrTestNewCode3, mesherr.None, []string{fmt.Sprintf("This error: %v", err), "line12"}, []string{"line21", "line22"}, []string{}, []string{"line41"}) //nolint:staticcheck
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
	countExpected := 3
	found := len(errors)
	if found != countExpected {
		t.Errorf("found %v call expressions; want %d", found, countExpected)
	}
	for _, e := range errors {
		if e.Name != "ErrTestNewCode1" && e.Name != "ErrTestNewCode2" && e.Name != "ErrTestNewCode3" {
			t.Errorf("invalid error name found: %s; want %s", e.Name, "ErrTestNewCode1 or ErrTestNewCode2")
		}
		if e.Name == "ErrTestNewCode1" {
			if e.Severity != "Fatal" ||
				len(e.ShortDescription) != 0 ||
				len(e.LongDescription) != 0 ||
				len(e.ProbableCause) != 0 ||
				len(e.SuggestedRemediation) != 0 {
				t.Errorf("invalid error found: %v", e)
			}
		}
		if e.Name == "ErrTestNewCode2" {
			if e.Severity != "None" ||
				e.ShortDescription != "line11" ||
				e.LongDescription != "line21\nline22" ||
				len(e.ProbableCause) != 0 ||
				e.SuggestedRemediation != "line41" {
				t.Errorf("invalid error found: %v", e)
			}
		}
		if e.Name == "ErrTestNewCode3" {
			if e.Severity != "None" ||
				e.ShortDescription != "line12" ||
				e.LongDescription != "line21\nline22" ||
				len(e.ProbableCause) != 0 ||
				e.SuggestedRemediation != "line41" {
				t.Errorf("invalid error found: %v", e)
			}
		}
	}
}
