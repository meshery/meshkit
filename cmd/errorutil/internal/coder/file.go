package coder

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	errutilerr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	"golang.org/x/exp/slog"
)

func handleFile(path string, update bool, updateAll bool, infoAll *errutilerr.InfoAll, comp *component.Info) error {
	logger := slog.New(slog.HandlerOptions{}.NewJSONHandler(os.Stdout)).With("path", path)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	logger.Info("inspecting file", "update", update)
	anyValueChanged := false

	ast.Inspect(file, func(n ast.Node) bool {
		if packageID, ok := isNewDefaultCallExpr(n); ok {
			logger.Warn("Usage of deprecated function NewDefault detected.", "packageID", packageID)
			if !stringSliceContains(infoAll.DeprecatedNewDefault, path) {
				infoAll.DeprecatedNewDefault = append(infoAll.DeprecatedNewDefault, path)
			}
			// If a NewDefault call expression is detected, child-nodes are not inspected.
			// This would lead to duplicates detections in case of dot-import.
			return false
		}
		if newError, ok := isNewCallExpr(n); ok {
			name := newError.Name
			logger.Info("New.Error(...) call detected", "error code name", name)
			_, ok := infoAll.Errors[name]
			if !ok {
				infoAll.Errors[name] = []errutilerr.Error{}
			}
			infoAll.Errors[name] = append(infoAll.Errors[name], *newError)
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
		err = os.WriteFile(path, buf.Bytes(), 0600)
		if err != nil {
			return err
		}
	}
	return nil
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func isErrorCodeVarName(name string) bool {
	matched, _ := regexp.MatchString("^Err[A-Z].+Code$", name)
	return matched
}

func stringSliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
