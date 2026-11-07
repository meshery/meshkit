package coder

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"
	mesherrors "github.com/meshery/meshkit/errors"
	"github.com/sirupsen/logrus"

	errutilerr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
)

// isNewDefaultCallExpr tests whether the node is a call expression for NewDefault.
// It returns the package identifier for convenience, empty for a dot-import.
// It does not verify that the actual type is from github.com/meshery/meshkit/errors.
func isNewDefaultCallExpr(node ast.Node) (string, bool) {
	if ce, ok := node.(*ast.CallExpr); ok {
		pkg, name, ok2 := isSelectorOrIdent(ce.Fun)
		if ok2 && name == "NewDefault" {
			return pkg, true
		}
	}
	return "", false
}

// isSelectorOrIdent checks whether a node is either a SelectorExpr or an Ident. They are both used in CallExpr to refer to a function.
// Ident is used for dot-imports. isSelectorOrIdent returns the package name (empty for a dot-import), the Ident name, and a boolean
// indication whether the is SelectorExpr or Ident.
func isSelectorOrIdent(node ast.Node) (string, string, bool) {
	switch node := node.(type) {
	case *ast.SelectorExpr:
		switch p := node.X.(type) {
		case *ast.Ident:
			return p.Name, node.Sel.Name, true
		}
	case *ast.Ident:
		return "", node.Name, true
	}
	return "", "", false
}

// stringArrayLines checks whether the node is a []string composite literal, and returns its string literal
// elements if it is. Elements that are not string literals (call expressions, identifiers, concatenations) are
// skipped: their text is only known at run time.
func stringArrayLines(node ast.Node) ([]string, bool) {
	cl, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	at, ok := cl.Type.(*ast.ArrayType)
	if !ok {
		return nil, false
	}
	if e, ok := at.Elt.(*ast.Ident); !ok || e.Name != "string" {
		return nil, false
	}
	var lines []string
	for _, elt := range cl.Elts {
		if s, ok := stringLiteral(elt); ok {
			lines = append(lines, s)
		}
	}
	return lines, true
}

// nonLiteralLines returns a problem for each element of the []string literal node that is not a string literal.
func nonLiteralLines(node ast.Node, field string) []string {
	var problems []string
	for i, elt := range node.(*ast.CompositeLit).Elts {
		if _, ok := stringLiteral(elt); !ok {
			problems = append(problems, fmt.Sprintf("%s[%d]: use a string literal; append instance text to LongDescription after NewV3 returns", field, i))
		}
	}
	return problems
}

// stringLiteral returns the value of a string literal node, unquoted.
func stringLiteral(node ast.Node) (string, bool) {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return strings.Trim(lit.Value, "\"`"), true
	}
	return s, true
}

// constructorArity is the number of arguments of each MeshKit error constructor errorutil reads.
var constructorArity = map[string]int{"New": 6, "NewV2": 7, "NewV3": 5}

// parseNewCallExpr checks whether node is an errors.New(...), errors.NewV2(...) or errors.NewV3(...) call and
// returns the error information if so. remediations resolves identifiers naming package-level errors.Remediation
// variables, the way a shared remediation is written once and referenced from several causes; nil resolves none.
func parseNewCallExpr(node ast.Node, remediations remediationResolver) (*errutilerr.Error, bool) {
	empty := &errutilerr.Error{}
	ce, ok := node.(*ast.CallExpr)
	if !ok {
		return empty, false
	}
	_, name, ok := isSelectorOrIdent(ce.Fun)
	if !ok {
		return empty, false
	}
	arity, known := constructorArity[name]
	args := ce.Args
	if !known || len(args) != arity {
		return empty, false
	}
	_, codeName, ok := isSelectorOrIdent(args[0])
	if !ok {
		return empty, false
	}
	_, severityName, ok := isSelectorOrIdent(args[1])
	if !ok {
		return empty, false
	}
	sDesc, ok := stringArrayLines(args[2])
	if !ok {
		return empty, false
	}
	lDesc, ok := stringArrayLines(args[3])
	if !ok {
		return empty, false
	}
	e := &errutilerr.Error{
		Name:        codeName,
		Code:        "", // Code is added after the whole code base is parsed
		Severity:    severityName,
		Constructor: name,
	}
	var cause, remedy []string
	if name == "NewV3" {
		causes, isCauses, problems := causesLiteral(args[4], remediations)
		if !isCauses {
			return empty, false
		}
		e.Problems = append(nonLiteralLines(args[2], "shortDescription"), nonLiteralLines(args[3], "longDescription")...)
		e.Problems = append(e.Problems, problems...)
		cause, remedy = mesherrors.FlattenCauses(causes)
		if len(problems) == 0 {
			if err := mesherrors.ValidateCauses(causes); err != nil {
				e.Problems = append(e.Problems, strings.Split(err.Error(), "\n")...)
			}
		}
		if len(e.Problems) == 0 {
			e.ProbableCauses = mesherrors.CausesToSchema(causes)
		}
	} else {
		if cause, ok = stringArrayLines(args[4]); !ok {
			return empty, false
		}
		if remedy, ok = stringArrayLines(args[5]); !ok {
			return empty, false
		}
	}
	e.ShortDescription = strings.Join(sDesc, "\n")
	e.LongDescription = strings.Join(lDesc, "\n")
	e.ProbableCause = strings.Join(cause, "\n")
	e.SuggestedRemediation = strings.Join(remedy, "\n")
	e.Lines = errutilerr.Lines{ShortDescription: sDesc, LongDescription: lDesc, ProbableCause: cause, SuggestedRemediation: remedy}
	return e, true
}

// handleValueSpec inspects node n if it is a ValueSpec, analyzes and updates it (depending on update and updateAll).
// Returns true if any value was changed.
func handleValueSpec(n ast.Node, update bool, updateAll bool, comp *component.Info, logger *logrus.Entry, path string, infoAll *errutilerr.InfoAll) bool {
	anyValueChanged := false
	spec, ok := n.(*ast.ValueSpec)
	if ok {
		for _, id := range spec.Names {
			if isErrorCodeVarName(id.Name) {
				value0 := id.Obj.Decl.(*ast.ValueSpec).Values[0]
				isLiteral := false
				isInteger := false
				oldValue := ""
				newValue := ""
				switch value := value0.(type) {
				case *ast.BasicLit:
					isLiteral = true
					oldValue = strings.Trim(strings.Trim(value.Value, "\""), fmt.Sprintf("%s-", comp.Name))
					isInteger = isInt(oldValue)
					if (update && !isInteger) || (update && updateAll) {
						value.Value = fmt.Sprintf("\"%s-%s\"", comp.Name, comp.GetNextErrorCode())
						newValue = strings.Trim(value.Value, "\"")
						anyValueChanged = true
						logger.WithFields(logrus.Fields{"name": id.Name, "value": newValue, "oldValue": oldValue}).Info("Err* variable with literal value replaced.")
					} else {
						newValue = oldValue
						logger.WithFields(logrus.Fields{"name": id.Name, "value": oldValue}).Info("Err* variable detected with literal value.")
					}
				case *ast.CallExpr:
					logger.WithFields(logrus.Fields{"name": id.Name}).Warn("Err* variable detected with call expression value.")
				}
				ec := &errutilerr.Info{
					Name:          id.Name,
					OldCode:       oldValue,
					Code:          newValue,
					CodeIsLiteral: isLiteral,
					CodeIsInt:     isInteger,
					Path:          path,
				}
				infoAll.Entries = append(infoAll.Entries, *ec)
				if isLiteral {
					key := oldValue
					if oldValue == "" {
						key = "no_code"
					}
					_, ok := infoAll.LiteralCodes[key]
					if !ok {
						infoAll.LiteralCodes[key] = []errutilerr.Info{}
					}
					infoAll.LiteralCodes[key] = append(infoAll.LiteralCodes[key], *ec)
				} else {
					infoAll.CallExprCodes = append(infoAll.CallExprCodes, *ec)
				}
			}
		}
	}
	return anyValueChanged
}
