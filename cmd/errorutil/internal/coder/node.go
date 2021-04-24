package coder

import (
	"go/ast"
)

// isNewDefaultCallExpr tests whether the node is a call expression for NewDefault.
// It returns the package identifier for convenience, empty for a dot-import.
// It does not verify that the actual type is from github.com/layer5io/meshkit/errors.
func isNewDefaultCallExpr(node ast.Node) (string, bool) {
	funcname := "NewDefault"
	switch node := node.(type) {
	case *ast.CallExpr:
		switch node := node.Fun.(type) {
		case *ast.SelectorExpr:
			switch p := node.X.(type) {
			case *ast.Ident:
				return p.Name, funcname == node.Sel.Name
			}
		}
	case *ast.Ident:
		return "", funcname == node.Name
	}
	return "", false
}
