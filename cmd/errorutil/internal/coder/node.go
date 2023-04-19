package coder

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	"golang.org/x/exp/slog"

	errutilerr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
)

// isNewDefaultCallExpr tests whether the node is a call expression for NewDefault.
// It returns the package identifier for convenience, empty for a dot-import.
// It does not verify that the actual type is from github.com/layer5io/meshkit/errors.
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

// isStringArray checks whether the node is a string array, and returns values joined with a \n if it is.
func isStringArray(node ast.Node) (string, bool) {
	isStringArr := false
	str := ""
	switch node := node.(type) {
	case *ast.CompositeLit:
		switch t := node.Type.(type) {
		case *ast.ArrayType:
			switch e := t.Elt.(type) {
			case *ast.Ident:
				isStringArr = e.Name == "string"
			}
		}
		if isStringArr && node.Elts != nil {
			var arr []string
			for _, elt := range node.Elts {
				switch e := elt.(type) {
				case *ast.BasicLit:
					arr = append(arr, strings.Trim(e.Value, "\""))
				}
			}
			str = strings.Join(arr, "\n")
		}
	}
	return str, isStringArr
}

// isNewCallExpr checks whether node is a errors.New(...) call and returns the error information if so.
func isNewCallExpr(node ast.Node) (*errutilerr.Error, bool) {
	if ce, ok := node.(*ast.CallExpr); ok {
		_, name, ok2 := isSelectorOrIdent(ce.Fun)
		if ok2 && name == "New" {
			// Check the signature:
			args := ce.Args
			if len(args) != 6 {
				return nil, false
			}

			_, codeName, ok := isSelectorOrIdent(args[0])
			if !ok {
				return nil, false
			}

			_, severityName, ok := isSelectorOrIdent(args[1])
			if !ok {
				return nil, false
			}

			shortDesc, ok := isStringArray(args[2])
			if !ok {
				return nil, false
			}

			longDesc, ok := isStringArray(args[3])
			if !ok {
				return nil, false
			}

			probCause, ok := isStringArray(args[4])
			if !ok {
				return nil, false
			}

			sugRemedy, ok := isStringArray(args[5])
			if !ok {
				return nil, false
			}

			return &errutilerr.Error{
				Name:                 codeName,
				Code:                 "", // Code is added after the whole code base is parsed
				Severity:             severityName,
				ShortDescription:     shortDesc,
				LongDescription:      longDesc,
				ProbableCause:        probCause,
				SuggestedRemediation: sugRemedy,
			}, true
		}
	}
	return nil, false
}

// handleValueSpec inspects node n if it is a ValueSpec, analyzes and updates it (depending on update and updateAll).
// Returns true if any value was changed.
func handleValueSpec(n ast.Node, update bool, updateAll bool, comp *component.Info, logger *slog.Logger, path string, infoAll *errutilerr.InfoAll) bool {
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
					oldValue = strings.Trim(value.Value, "\"")
					isInteger = isInt(oldValue)
					if (update && !isInteger) || (update && updateAll) {
						value.Value = fmt.Sprintf("\"%s\"", comp.GetNextErrorCode())
						newValue = strings.Trim(value.Value, "\"")
						anyValueChanged = true
						logger.Info("Err* variable with literal value replaced.", "name", id.Name, "value", newValue, "oldValue", oldValue)
					} else {
						newValue = oldValue
						logger.Info("Err* variable detected with literal value.", "name", id.Name, "value", oldValue)
					}
				case *ast.CallExpr:
					logger.Warn("Err* variable detected with call expression value.", "name", id.Name)
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
