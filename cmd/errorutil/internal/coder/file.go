package coder

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	errutilerr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	"github.com/sirupsen/logrus"
)

func handleFile(path string, update bool, updateAll bool, infoAll *errutilerr.InfoAll, comp *component.Info) error {
	logger := logrus.WithFields(logrus.Fields{"path": path})
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{"update": update}).Info("inspecting file")
	ast.Inspect(file, func(n ast.Node) bool {
		if pgkid, ok := isNewDefaultCallExpr(n); ok {
			logger.Warnf("Usage of deprecated function %s.NewDefault detected.", pgkid)
			if !contains(infoAll.DeprecatedNewDefault, path) {
				infoAll.DeprecatedNewDefault = append(infoAll.DeprecatedNewDefault, path)
			}
			// If a NewDefault call expression is detected, child-nodes are not inspected.
			// This would lead to duplicates detections in case of dot-import.
			return false
		}
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
		return true
	})
	if update {
		logger.Info("writing updated file")
		buf := new(bytes.Buffer)
		err = format.Node(buf, fset, file)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(path, buf.Bytes(), 0600)
		if err != nil {
			return err
		}
	}
	return nil
}

func isErrorCodeVarName(name string) bool {
	matched, _ := regexp.MatchString("^Err[A-Z]", name)
	return matched
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
