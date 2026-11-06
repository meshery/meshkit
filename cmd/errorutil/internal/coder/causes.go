package coder

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"

	mesherrors "github.com/meshery/meshkit/errors"
)

// remediationResolver resolves the name of a package-level errors.Remediation variable to its value, so a
// remediation shared by several causes can be written once in source and referenced from each cause.
// ok is false when the name is not such a variable.
type remediationResolver func(name string) (rem mesherrors.Remediation, problems []string, ok bool)

// remediationVar is one package-level `var name = errors.Remediation{...}` declaration.
type remediationVar struct {
	rem      mesherrors.Remediation
	problems []string
}

// remediationVars holds the package-level errors.Remediation variables of each directory (Go package) by name.
type remediationVars map[string]map[string]remediationVar

// resolverFor returns the resolver for the package in dir.
func (v remediationVars) resolverFor(dir string) remediationResolver {
	vars := v[dir]
	return func(name string) (mesherrors.Remediation, []string, bool) {
		rv, ok := vars[name]
		return rv.rem, rv.problems, ok
	}
}

// collectRemediationVars records the package-level errors.Remediation variables declared in the Go file at path.
func collectRemediationVars(path string, vars remediationVars) error {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, name := range vs.Names {
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok || !isTypeNamed(cl.Type, "Remediation") {
					continue
				}
				rem, problems := remediationLiteral(cl, fmt.Sprintf("var %s", name.Name))
				if vars[dir] == nil {
					vars[dir] = map[string]remediationVar{}
				}
				vars[dir][name.Name] = remediationVar{rem: rem, problems: problems}
			}
		}
	}
	return nil
}

// isTypeNamed reports whether expr names the type typeName, qualified (errors.Remediation) or dot-imported.
func isTypeNamed(expr ast.Expr, typeName string) bool {
	_, name, ok := isSelectorOrIdent(expr)
	return ok && name == typeName
}

// causesLiteral reads the causes argument of an errors.NewV3 call: a []errors.Cause composite literal written
// with string literals. isCauses is false when node is not such a literal, so the call is not a NewV3 definition
// errorutil can read. problems lists everything that keeps the causes from being exported as written.
func causesLiteral(node ast.Node, resolve remediationResolver) (causes []mesherrors.Cause, isCauses bool, problems []string) {
	cl, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil, false, nil
	}
	at, ok := cl.Type.(*ast.ArrayType)
	if !ok || !isTypeNamed(at.Elt, "Cause") {
		return nil, false, nil
	}
	for i, elt := range cl.Elts {
		path := fmt.Sprintf("causes[%d]", i)
		ccl, ok := elt.(*ast.CompositeLit)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: write each cause as an errors.Cause literal", path))
			continue
		}
		cause, causeProblems := causeLiteral(ccl, path, resolve)
		problems = append(problems, causeProblems...)
		causes = append(causes, cause)
	}
	return causes, true, problems
}

func causeLiteral(cl *ast.CompositeLit, path string, resolve remediationResolver) (mesherrors.Cause, []string) {
	var cause mesherrors.Cause
	var problems []string
	fields, fieldProblems := keyedFields(cl, path)
	problems = append(problems, fieldProblems...)
	for _, key := range sortedKeys(fields) {
		value := fields[key]
		switch key {
		case "ID", "Description":
			s, ok := stringLiteral(value)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s.%s: use a string literal", path, key))
				continue
			}
			if key == "ID" {
				cause.ID = s
			} else {
				cause.Description = s
			}
		case "Remediations":
			rcl, ok := value.(*ast.CompositeLit)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s.Remediations: write the remediations as an []errors.Remediation literal", path))
				continue
			}
			for j, relt := range rcl.Elts {
				rpath := fmt.Sprintf("%s.remediations[%d]", path, j)
				switch r := relt.(type) {
				case *ast.CompositeLit:
					rem, remProblems := remediationLiteral(r, rpath)
					problems = append(problems, remProblems...)
					cause.Remediations = append(cause.Remediations, rem)
				case *ast.Ident:
					var rem mesherrors.Remediation
					var remProblems []string
					ok := false
					if resolve != nil {
						rem, remProblems, ok = resolve(r.Name)
					}
					if !ok {
						problems = append(problems, fmt.Sprintf("%s: %s is not a package-level errors.Remediation variable", rpath, r.Name))
						continue
					}
					problems = append(problems, remProblems...)
					cause.Remediations = append(cause.Remediations, rem)
				default:
					problems = append(problems, fmt.Sprintf("%s: write the remediation as an errors.Remediation literal or a package-level errors.Remediation variable", rpath))
				}
			}
		default:
			problems = append(problems, fmt.Sprintf("%s: unknown field %s", path, key))
		}
	}
	return cause, problems
}

// policyNames maps the identifiers of MeshKit's ActionPolicy constants to their values.
var policyNames = map[string]mesherrors.ActionPolicy{
	"Automatic":        mesherrors.Automatic,
	"RequiresApproval": mesherrors.RequiresApproval,
	"Manual":           mesherrors.Manual,
}

func remediationLiteral(cl *ast.CompositeLit, path string) (mesherrors.Remediation, []string) {
	var rem mesherrors.Remediation
	fields, problems := keyedFields(cl, path)
	for _, key := range sortedKeys(fields) {
		value := fields[key]
		switch key {
		case "ID", "Description", "Action":
			s, ok := stringLiteral(value)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s.%s: use a string literal", path, key))
				continue
			}
			switch key {
			case "ID":
				rem.ID = s
			case "Description":
				rem.Description = s
			default:
				rem.Action = s
			}
		case "ActionPolicy":
			if s, ok := stringLiteral(value); ok {
				rem.ActionPolicy = mesherrors.ActionPolicy(s)
				continue
			}
			_, name, ok := isSelectorOrIdent(value)
			policy, known := policyNames[name]
			if !ok || !known {
				problems = append(problems, fmt.Sprintf("%s.ActionPolicy: use errors.Automatic, errors.RequiresApproval or errors.Manual", path))
				continue
			}
			rem.ActionPolicy = policy
		default:
			problems = append(problems, fmt.Sprintf("%s: unknown field %s", path, key))
		}
	}
	if rem.ActionPolicy == "" {
		rem.ActionPolicy = mesherrors.Manual // as errors.NewV3 stores it
	}
	return rem, problems
}

// keyedFields returns a composite literal's fields by key, reporting unkeyed fields, which errorutil cannot map.
func keyedFields(cl *ast.CompositeLit, path string) (map[string]ast.Expr, []string) {
	fields := map[string]ast.Expr{}
	var problems []string
	for i, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: field %d is unkeyed; name every field", path, i))
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: field %d has an unexpected key", path, i))
			continue
		}
		fields[key.Name] = kv.Value
	}
	return fields, problems
}

func sortedKeys(m map[string]ast.Expr) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// collectAllRemediationVars walks the same files as walk and records their package-level errors.Remediation
// variables.
func collectAllRemediationVars(globalFlags globalFlags, subDirsToSkip []string) (remediationVars, error) {
	vars := remediationVars{}
	err := filepath.Walk(globalFlags.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && contains(subDirsToSkip, info.Name()) {
			return filepath.SkipDir
		}
		if !info.IsDir() && includeFile(path) {
			return collectRemediationVars(path, vars)
		}
		return nil
	})
	return vars, err
}
