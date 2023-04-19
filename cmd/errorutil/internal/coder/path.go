package coder

import (
	"os"
	"path/filepath"
	"strings"

	meshlogger "github.com/layer5io/meshkit/cmd/errorutil/logger"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	errutilerr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	"golang.org/x/exp/slog"
)

var logger = slog.New(slog.HandlerOptions{}.NewJSONHandler(os.Stdout))

func walk(globalFlags globalFlags, update bool, updateAll bool, errorsInfo *errutilerr.InfoAll) error {
	subDirsToSkip := append([]string{".git", ".github"}, globalFlags.skipDirs...)
	meshlogger.Infof(logger, "root directory: %s", globalFlags.rootDir)
	meshlogger.Infof(logger, "output directory: %s", globalFlags.outDir)
	meshlogger.Infof(logger, "info directory: %s", globalFlags.infoDir)
	meshlogger.Infof(logger, "subdirs to skip: %v", subDirsToSkip)
	comp, err := component.New(globalFlags.infoDir)
	if err != nil {
		return err
	}

	err = filepath.Walk(globalFlags.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("failure accessing path: %v", err)
			return err
		}
		if info.IsDir() && stringSliceContains(subDirsToSkip, info.Name()) {
			meshlogger.Infof(logger, "skipping directory %s", info.Name())
			return filepath.SkipDir
		}
		if info.IsDir() {
			logger.Debug("handling dir")
		} else {
			if includeFile(path) {
				isErrorsGoFile := isErrorGoFile(path)
				logger.Debug("handling Go file: iserrorsfile=%v", isErrorsGoFile)
				err := handleFile(path, update && isErrorsGoFile, updateAll, errorsInfo, comp)
				if err != nil {
					return err
				}
			} else {
				logger.Debug("skipping file")
			}
		}
		return nil
	})
	if update {
		err = comp.Write()
	}
	return err
}

func includeFile(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return false
	}
	if filepath.Ext(path) == ".go" {
		return true
	}
	return false
}

func isErrorGoFile(path string) bool {
	_, file := filepath.Split(path)
	return file == "error.go"
}
