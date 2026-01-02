package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// CallerHook adds real caller information to log entries
type CallerHook struct {
	skippedPaths []string
}

// defaultCallerSkippedPaths contains the default function name patterns to skip when finding the real caller
var (
	defaultCallerSkippedPaths = []string{
		"github.com/meshery/meshkit/logger",
		"github.com/sirupsen/logrus",
	}
	defaultCallerSkippedPathsMu sync.RWMutex
)

// SetDefaultCallerSkippedPaths sets the default skipped paths on a package level
func SetDefaultCallerSkippedPaths(paths []string) {
	defaultCallerSkippedPathsMu.Lock()
	defer defaultCallerSkippedPathsMu.Unlock()
	defaultCallerSkippedPaths = paths
}

// Levels returns the levels this hook should be applied to
func (hook *CallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// shouldSkipFrame checks if a function should be skipped based on function name
func (hook *CallerHook) shouldSkipFrame(pc uintptr) bool {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return false
	}

	funcName := fn.Name()
	for _, path := range hook.skippedPaths {
		if strings.Contains(funcName, path) {
			return true
		}
	}
	return false
}

// Fire adds caller information to the log entry
func (hook *CallerHook) Fire(entry *logrus.Entry) error {
	// Skip frames to get to the real caller (outside skipped packages)
	for i := range 16 {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		if !hook.shouldSkipFrame(pc) {
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
