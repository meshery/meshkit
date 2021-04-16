package coder

import (
	"fmt"
	"os"
	"path/filepath"

	mesherr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
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

func walk(rootDir string, update bool, updateAll bool, errorsInfo *mesherr.ErrorsInfo) {
	subDirsToSkip := []string{".git", ".github"}
	logrus.Info(fmt.Sprintf("root directory: %s", rootDir))
	logrus.Info(fmt.Sprintf("subdirs to skip: %v", subDirsToSkip))

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		logger := logrus.WithFields(logrus.Fields{"path": path})
		if err != nil {
			logger.WithFields(logrus.Fields{"error": fmt.Sprintf("%v", err)}).Warn("failure accessing path")
			return err
		}
		if info.IsDir() && contains(subDirsToSkip, info.Name()) {
			logger.Debug("skipping dir")
			return filepath.SkipDir
		}
		if info.IsDir() {
			logger.Debug("handling dir")
		} else {
			if filepath.Ext(path) == ".go" {
				isErrorsGoFile := IsErrorGoFile(path)
				logger.WithFields(logrus.Fields{"iserrorsfile": fmt.Sprintf("%v", isErrorsGoFile)}).Debug("handling Go file")
				err := handleFile(path, update, updateAll, errorsInfo)
				if err != nil {
					logger.Errorf("error on analyze: %v", err)
				}
			} else {
				logger.Debug("skipping file")
			}
		}
		return nil
	})
	if err != nil {
		logrus.Error(fmt.Sprintf("error walking the path %q: %v\n", rootDir, err))
		return
	}
}

func IsErrorGoFile(path string) bool {
	_, file := filepath.Split(path)
	return file == "error.go"
}
