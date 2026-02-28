package logger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	meshkitError "github.com/meshery/meshkit/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type dummyError struct{}

func (d dummyError) Error() string { return "dummy error" }

var mError = meshkitError.New(
	"code",
	meshkitError.Alert,
	[]string{"short test error occurred"},
	[]string{"long test error occurred in the function X when doing Y"},
	[]string{"the probable cause of the error is Z"},
	[]string{"try doing A, B, or C to remediate the error"},
)

func TestLogger_Info_Debug_Warn_Error_Fatal(t *testing.T) {
	var outBuffer, errBuffer bytes.Buffer
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.DebugLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	// Redirect output for testing
	l.UpdateLogOutput(&outBuffer)
	l.UpdateErrorLogOutput(&errBuffer)

	l.Info("info message")
	assert.Contains(t, outBuffer.String(), "info message")
	outBuffer.Reset()

	l.Infof("infof %s", "message")
	assert.Contains(t, outBuffer.String(), "infof message")
	outBuffer.Reset()

	l.Debug("debug message")
	assert.Contains(t, outBuffer.String(), "debug message")
	outBuffer.Reset()

	l.Debugf("debugf %s", "message")
	assert.Contains(t, outBuffer.String(), "debugf message")
	outBuffer.Reset()

	l.Warn(errors.New("warn error"))
	assert.Contains(t, outBuffer.String(), "warn error")
	outBuffer.Reset()

	l.Warnf("warnf %s", "message")
	assert.Contains(t, outBuffer.String(), "warnf message")
	outBuffer.Reset()

	l.Error(errors.New("error message"))
	assert.Contains(t, errBuffer.String(), "error message")
	errBuffer.Reset()

	l.Errorf("errorf %s", "message")
	assert.Contains(t, errBuffer.String(), "errorf message")
	errBuffer.Reset()
}

func TestLogger_SetLevel_GetLevel(t *testing.T) {
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	l.SetLevel(logrus.WarnLevel)
	assert.Equal(t, logrus.WarnLevel, l.GetLevel())
}

func TestLogger_UpdateLogOutput(t *testing.T) {
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	var buf bytes.Buffer
	l.UpdateLogOutput(&buf)
	l.Info("output test")
	assert.Contains(t, buf.String(), "output test")
}

func TestLogger_UpdateErrorLogOutput(t *testing.T) {
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	var buf bytes.Buffer
	l.UpdateErrorLogOutput(&buf)
	l.Error(errors.New("error output test"))
	assert.Contains(t, buf.String(), "error output test")
}

func TestTerminalFormatter_Format(t *testing.T) {
	formatter := &TerminalFormatter{}
	entry := &logrus.Entry{
		Message: "test message",
		Data:    logrus.Fields{},
	}
	b, err := formatter.Format(entry)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "test message")

	entry.Data["caller"] = "main.go:10"
	b, err = formatter.Format(entry)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "[main.go:10] test message")
}

func TestLogger_Error_MeshkitError(t *testing.T) {
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	var buf bytes.Buffer
	l.UpdateErrorLogOutput(&buf)

	l.Error(mError)
	assert.Contains(t, buf.String(), "short test error occurred")
	assert.Contains(t, buf.String(), "long test error occurred in the function X when doing Y")
	assert.Contains(t, buf.String(), "the probable cause of the error is Z")
	assert.Contains(t, buf.String(), "try doing A, B, or C to remediate the error")
}

func TestLoggerIntegration_FileOutput(t *testing.T) {
	tmpDir := os.TempDir()
	logFile := filepath.Join(tmpDir, "meshkit_logger_integration.log")
	f, err := os.Create(logFile)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.Remove(logFile)
	}()

	logger, err := New("testapp", Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.InfoLevel),
		EnableCallerInfo: false,
		Output:           f,
	})
	assert.NoError(t, err)

	msg := "integration test log entry"
	logger.Info(msg)

	err = f.Sync()
	assert.NoError(t, err)

	data, err := os.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Contains(t, string(data), msg)

	logger.Error(mError)

	err = f.Sync()
	assert.NoError(t, err)

	data, err = os.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "short test error occurred")
	assert.Contains(t, string(data), "long test error occurred in the function X when doing Y")
	assert.Contains(t, string(data), "the probable cause of the error is Z")
	assert.Contains(t, string(data), "try doing A, B, or C to remediate the error")
}
