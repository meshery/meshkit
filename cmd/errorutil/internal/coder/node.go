package coder

import (
	"go/ast"
	"strings"

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
	empty := &errutilerr.Error{}
	if ce, ok := node.(*ast.CallExpr); ok {
		_, name, ok2 := isSelectorOrIdent(ce.Fun)
		if ok2 && name == "New" {
			// check the signature:
			args := ce.Args
			if len(args) != 6 {
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
			sDesc, ok := isStringArray(args[2])
			if !ok {
				return empty, false
			}
			lDesc, ok := isStringArray(args[3])
			if !ok {
				return empty, false
			}
			cause, ok := isStringArray(args[4])
			if !ok {
				return empty, false
			}
			remedy, ok := isStringArray(args[5])
			if !ok {
				return empty, false
			}
			return &errutilerr.Error{
				Name:                 codeName,
				Code:                 "", // Code is added after the whole code base is parsed
				Severity:             severityName,
				ShortDescription:     sDesc,
				LongDescription:      lDesc,
				ProbableCause:        cause,
				SuggestedRemediation: remedy,
			}, true
		}
	}
	return empty, false
}
