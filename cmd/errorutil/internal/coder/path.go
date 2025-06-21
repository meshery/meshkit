package coder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"

	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
	"github.com/sirupsen/logrus"
)

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func walk(globalFlags globalFlags, update bool, updateAll bool, errorsInfo *mesherr.InfoAll) error {
	subDirsToSkip := append([]string{".git", ".github"}, globalFlags.skipDirs...)
	logrus.Infof("root directory: %s", globalFlags.rootDir)
	logrus.Infof("output directory: %s", globalFlags.outDir)
	logrus.Infof("info directory: %s", globalFlags.infoDir)
	logrus.Infof("subdirs to skip: %v", subDirsToSkip)
	comp, err := component.New(globalFlags.infoDir)
	if err != nil {
		return err
	}

	err = filepath.Walk(globalFlags.rootDir, func(path string, info os.FileInfo, err error) error {
		logger := logrus.WithFields(logrus.Fields{"path": path})
		if err != nil {
			logger.WithFields(logrus.Fields{"error": fmt.Sprintf("%v", err)}).Warn("failure accessing path")
			return err
		}
		if info.IsDir() && contains(subDirsToSkip, info.Name()) {
			logger.Infof("skipping directory %s", info.Name())
			return filepath.SkipDir
		}
		if info.IsDir() {
			logger.Debug("handling dir")
		} else {
			if includeFile(path) {
				isErrorsGoFile := isErrorGoFile(path)
				logger.WithFields(logrus.Fields{"iserrorsfile": fmt.Sprintf("%v", isErrorsGoFile)}).Debug("handling Go file")
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

func isErrorGoFile(path string) bool {
	_, file := filepath.Split(path)
	return file == "error.go"
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
