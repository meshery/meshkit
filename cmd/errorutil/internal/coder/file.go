package coder

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	mesherr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	"github.com/sirupsen/logrus"
)

func handleFile(path string, update bool, updateAll bool, errorsInfo *mesherr.ErrorsInfo) error {
	logger := logrus.WithFields(logrus.Fields{"path": path})
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{"update": update}).Info("inspecting file")
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if ok {
			for _, id := range spec.Names {
				if IsErrorCodeName(id.Name) {
					value0 := id.Obj.Decl.(*ast.ValueSpec).Values[0]
					isLiteral := false
					isInteger := false
					oldValue := ""
					newValue := ""
					switch value := value0.(type) {
					case *ast.BasicLit:
						isLiteral = true
						oldValue = strings.Trim(value.Value, "\"")
						isInteger = IsInt(oldValue)
						if (update && !isInteger) || (update && updateAll) {
							value.Value = component.GetNextErrorCode()
							newValue = strings.Trim(value.Value, "\"")
							logger.WithFields(logrus.Fields{"name": id.Name, "value": newValue, "oldValue": oldValue}).Info("Err* variable with literal value replaced.")
						} else {
							newValue = oldValue
							logger.WithFields(logrus.Fields{"name": id.Name, "value": oldValue}).Info("Err* variable detected with literal value.")
						}
					case *ast.CallExpr:
						logger.WithFields(logrus.Fields{"name": id.Name}).Warn("Err* variable detected with call expression value.")
					}
					ec := &mesherr.ErrorInfo{
						Name:          id.Name,
						OldCode:       oldValue,
						Code:          newValue,
						CodeIsLiteral: isLiteral,
						CodeIsInt:     isInteger,
						Path:          path,
					}
					errorsInfo.Entries = append(errorsInfo.Entries, *ec)
					if isLiteral {
						key := oldValue
						if oldValue == "" {
							key = "no_code"
						}
						_, ok := errorsInfo.LiteralCodes[key]
						if !ok {
							errorsInfo.LiteralCodes[key] = []mesherr.ErrorInfo{}
						}
						errorsInfo.LiteralCodes[key] = append(errorsInfo.LiteralCodes[key], *ec)
					} else {
						errorsInfo.CallExprCodes = append(errorsInfo.CallExprCodes, *ec)
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

func IsErrorCodeName(name string) bool {
	matched, _ := regexp.MatchString("^Err[A-Z]", name)
	return matched
}

func IsInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
