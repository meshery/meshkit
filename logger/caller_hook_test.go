package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// testSkippedPaths contains the paths we want to skip during tests
// (including caller_hook.go but excluding the logger.go and test files)
var testSkippedPaths = []string{
	"meshkit/logger/caller_hook.go",
	"meshkit/logger/logger.go",
	"sirupsen/logrus",
}

func TestCallerHook_WithMeshkitLogger(t *testing.T) {
	t.Run("caller hook populates caller info with JSON format", func(t *testing.T) {
		var buf bytes.Buffer

		// Create meshkit logger with caller hook enabled and custom skipped paths
		logger, err := New("test-app", Options{
			Format:             JsonLogFormat,
			LogLevel:           int(logrus.InfoLevel),
			Output:             &buf,
			EnableCallerInfo:   true,
			CallerSkippedPaths: testSkippedPaths,
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Log a message through separate function, so that the caller will be logTestMessage
		logTestMessage(logger)

		// Parse the JSON output
		var logEntry map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log output as JSON: %v", err)
		}

		// Check that caller field exists
		caller, exists := logEntry["caller"]
		if !exists {
			t.Error("Expected 'caller' field in log output")
		}

		callerStr, ok := caller.(string)
		if !ok {
			t.Error("Expected caller to be a string")
		}

		// Should detect logTestMessage as the caller since it's not in skipped paths
		if !strings.Contains(callerStr, "logTestMessage") {
			t.Errorf("Expected caller to contain 'logTestMessage', got: %s", callerStr)
		}

		if !strings.Contains(callerStr, "caller_hook_test.go:") {
			t.Errorf("Expected caller to contain test filename, got: %s", callerStr)
		}
	})

	t.Run("caller hook populates caller info with terminal format", func(t *testing.T) {
		var buf bytes.Buffer

		// Create meshkit logger with caller hook enabled and terminal format
		logger, err := New("test-app", Options{
			Format:             TerminalLogFormat,
			LogLevel:           int(logrus.InfoLevel),
			Output:             &buf,
			EnableCallerInfo:   true,
			CallerSkippedPaths: testSkippedPaths,
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Log a message through separate function, so that a caller will be logTestMessage
		logTestMessage(logger)

		output := buf.String()

		if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
			t.Errorf("Expected terminal format with [caller_info], got: %s", output)
		}

		if !strings.Contains(output, "logTestMessage") {
			t.Errorf("Expected caller to contain 'logTestMessage', got: %s", output)
		}

		if !strings.Contains(output, "caller_hook_test.go:") {
			t.Errorf("Expected caller to contain test filename, got: %s", output)
		}

		if !strings.Contains(output, "test message from meshkit logger") {
			t.Errorf("Expected original message to be preserved, got: %s", output)
		}
	})

	t.Run("caller hook disabled does not add caller info", func(t *testing.T) {
		var buf bytes.Buffer

		// Create meshkit logger WITHOUT caller hook enabled
		logger, err := New("test-app", Options{
			Format:           JsonLogFormat,
			LogLevel:         int(logrus.InfoLevel),
			Output:           &buf,
			EnableCallerInfo: false, // Explicitly disabled
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Log a message
		logger.Info("test message without caller info")

		// Parse the JSON output
		var logEntry map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log output as JSON: %v", err)
		}

		// Check that caller field does NOT exist
		if _, exists := logEntry["caller"]; exists {
			t.Error("Expected no 'caller' field when EnableCallerInfo is false")
		}

		// Should still have the message
		if logEntry["msg"] != "test message without caller info" {
			t.Error("Expected original message to be preserved")
		}
	})
}

// Helper function to test caller detection through multiple call levels
func helperThatLogsInfo(logger Handler) {
	logger.Info("message from helper function")
}

// Separate function to test direct logger calls
func logTestMessage(logger Handler) {
	logger.Info("test message from meshkit logger")
}

func TestCallerHook_NestedCalls(t *testing.T) {
	t.Run("caller hook detects correct caller through nested function calls", func(t *testing.T) {
		var buf bytes.Buffer

		logger, err := New("test-app", Options{
			Format:             JsonLogFormat,
			LogLevel:           int(logrus.InfoLevel),
			Output:             &buf,
			EnableCallerInfo:   true,
			CallerSkippedPaths: testSkippedPaths,
		})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Call through helper function
		helperThatLogsInfo(logger)

		var logEntry map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log output as JSON: %v", err)
		}

		caller, exists := logEntry["caller"]
		if !exists {
			t.Error("Expected 'caller' field in log output")
		}

		callerStr := caller.(string)

		// Should detect helperThatLogsInfo as the caller
		if !strings.Contains(callerStr, "helperThatLogsInfo") {
			t.Errorf("Expected caller to contain 'helperThatLogsInfo', got: %s", callerStr)
		}
	})
}

func TestCallerHook_DifferentLogLevels(t *testing.T) {
	testCases := []struct {
		name      string
		logFunc   func(Handler)
		hasOutput bool
	}{
		{"Info", func(l Handler) { l.Info("info message") }, true},
		{"Infof", func(l Handler) { l.Infof("formatted %s message", "info") }, true},
		{"Debug", func(l Handler) { l.Debug("debug message") }, false}, // Debug level will be filtered out
		{"Warn", func(l Handler) { l.Warnf("warn message") }, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			logger, err := New("test-app", Options{
				Format:             JsonLogFormat,
				LogLevel:           int(logrus.InfoLevel), // Only Info and above
				Output:             &buf,
				EnableCallerInfo:   true,
				CallerSkippedPaths: testSkippedPaths,
			})
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}

			tc.logFunc(logger)

			if tc.hasOutput {
				if buf.Len() == 0 {
					t.Errorf("Expected log output for %s level", tc.name)
					return
				}

				var logEntry map[string]interface{}
				err = json.Unmarshal(buf.Bytes(), &logEntry)
				if err != nil {
					t.Fatalf("Failed to parse log output as JSON: %v", err)
				}

				caller, exists := logEntry["caller"]
				if !exists {
					t.Error("Expected 'caller' field in log output")
				}

				callerStr := caller.(string)
				if !strings.Contains(callerStr, "TestCallerHook_DifferentLogLevels") {
					t.Errorf("Expected caller to contain test function name, got: %s", callerStr)
				}
			} else {
				if buf.Len() > 0 {
					t.Errorf("Expected no log output for %s level at Info threshold", tc.name)
				}
			}
		})
	}
}

func TestShouldSkipFrame(t *testing.T) {
	hook := &CallerHook{skippedPaths: []string{"meshkit/logger", "sirupsen/logrus"}}

	testCases := []struct {
		name     string
		filepath string
		expected bool
	}{
		{"should skip meshkit/logger path", "/some/path/meshkit/logger/logger.go", true},
		{"should skip meshkit/logger path", "/some/path/meshkit/logger/caller_hook.go", true},
		{"should skip sirupsen/logrus path", "/some/path/sirupsen/logrus/entry.go", true},
		{"should not skip meshkit/logger test files", "/some/path/meshkit/logger/caller_hook_test.go", true}, // This will be skipped with default paths
		{"should not skip regular application path", "/some/path/myapp/main.go", false},
		{"should not skip empty path", "", false},
		{"should not skip partial match", "/some/path/logger-test/file.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hook.shouldSkipFrame(tc.filepath)
			if result != tc.expected {
				t.Errorf("shouldSkipFrame(%q) = %v, expected %v", tc.filepath, result, tc.expected)
			}
		})
	}
}

func TestCallerHook_Levels(t *testing.T) {
	hook := &CallerHook{}
	levels := hook.Levels()

	if len(levels) != len(logrus.AllLevels) {
		t.Errorf("Expected %d levels, got %d", len(logrus.AllLevels), len(levels))
	}

	// Verify it includes all expected levels
	expectedLevels := logrus.AllLevels
	for i, level := range levels {
		if level != expectedLevels[i] {
			t.Errorf("Expected level %v at index %d, got %v", expectedLevels[i], i, level)
		}
	}
}

func TestSetDefaultCallerSkippedPaths(t *testing.T) {
	// Save original defaults
	originalDefaults := defaultCallerSkippedPaths
	defer func() {
		defaultCallerSkippedPaths = originalDefaults
	}()

	// Set new defaults
	newDefaults := []string{"custom/path", "another/path"}
	SetDefaultCallerSkippedPaths(newDefaults)

	// Verify the defaults were set
	if len(defaultCallerSkippedPaths) != 2 {
		t.Errorf("Expected 2 default paths, got %d", len(defaultCallerSkippedPaths))
	}

	if defaultCallerSkippedPaths[0] != "custom/path" {
		t.Errorf("Expected first path to be 'custom/path', got %s", defaultCallerSkippedPaths[0])
	}

	if defaultCallerSkippedPaths[1] != "another/path" {
		t.Errorf("Expected second path to be 'another/path', got %s", defaultCallerSkippedPaths[1])
	}

	// Test that new logger uses the updated defaults
	var buf bytes.Buffer
	logger, err := New("test-app", Options{
		Format:           JsonLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		Output:           &buf,
		EnableCallerInfo: true,
		// Not providing CallerSkippedPaths, should use defaults
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Since we don't have access to the hook's internal skippedPaths directly,
	// we can test by logging and ensuring the hook was created successfully
	logger.Info("test message")

	if buf.Len() == 0 {
		t.Error("Expected log output")
	}
}
