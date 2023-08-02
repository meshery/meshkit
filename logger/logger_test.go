package logger_test

import (
	"testing"

	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/logger"
)

var ErrCode = "10000"

func Err(err error, msg string) error {
	return errors.New(ErrCode, errors.Alert, []string{msg}, []string{err.Error()}, []string{}, []string{})
}

func TestLogger(t *testing.T) {

	t.Run("New", func(t *testing.T) {
		_, err := logger.New("app", logger.Options{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("LoggingLevels", func(t *testing.T) {
		logger, err := logger.New("app", logger.Options{})

		logger.Debug("debug")
		logger.Info("info")
		logger.Warn(err)
		logger.Error(err)

		logger.Debugf("debugf %d", 1)
		logger.Infof("infof %d", 2)
		logger.Warnf("warnf %d", 3)
		logger.Errorf("errorf %d", 4)
	})

	// Could add more test cases for different options,
	// log formats, error cases, etc.

}
