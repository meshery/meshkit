package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// CallerHook adds real caller information to log entries
type CallerHook struct{}

// skippedPaths contains path patterns to skip when finding the real caller
var skippedPaths = []string{
	"meshkit/logger",
	"sirupsen/logrus",
}

// Levels returns the levels this hook should be applied to
func (hook *CallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// shouldSkipFrame checks if a file path should be skipped
func shouldSkipFrame(file string) bool {
	for _, path := range skippedPaths {
		if strings.Contains(file, path) {
			return true
		}
	}
	return false
}

// Fire adds caller information to the log entry
func (hook *CallerHook) Fire(entry *logrus.Entry) error {
	// Skip frames to get to the real caller (outside skipped packages)
	for i := 0; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		if !shouldSkipFrame(file) {
			fn := runtime.FuncForPC(pc)
			funcName := "unknown"
			if fn != nil {
				funcName = fn.Name()
			}

			filename := filepath.Base(file)
			entry.Data["caller"] = fmt.Sprintf("%s %s:%d", funcName, filename, line)
			break
		}
	}
	return nil
}