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
	anyValueChanged := false
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
		if newErr, ok := isNewCallExpr(n); ok {
			name := newErr.Name
			logger.Infof("New.Error(...) call detected, error code name: '%s'", name)
			_, ok := infoAll.Errors[name]
			if !ok {
				infoAll.Errors[name] = []errutilerr.Error{}
			}
			infoAll.Errors[name] = append(infoAll.Errors[name], *newErr)
			// If a New call expression is detected, child-nodes are not inspected:
			return false
		}
		if handleValueSpec(n, update, updateAll, comp, logger, path, infoAll) {
			anyValueChanged = true
		}
		return true
	})
	if update && anyValueChanged {
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
	matched, _ := regexp.MatchString("^Err[A-Z].+Code$", name)
	return matched
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
