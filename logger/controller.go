package logger

import (
	"github.com/go-logr/logr"
	"github.com/layer5io/meshkit/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrControllerCode = "test"
)

func ErrController(err error, msg string) error {
	return errors.New(ErrControllerCode, errors.Alert, []string{msg}, []string{err.Error()}, []string{}, []string{})
}

type Controller struct {
	enabled bool
	base    *Logger
}

func (l *Logger) ControllerLogger() logr.Logger {
	return &Controller{
		enabled: true,
		base:    l,
	}
}

func (c *Controller) Enabled() bool {
	return c.enabled
}

func (c *Controller) Info(msg string, keysAndValues ...interface{}) {
	c.base.Info(msg)
}

func (c *Controller) Error(err error, msg string, keysAndValues ...interface{}) {
	c.base.Error(ErrController(err, msg))
}

func (c *Controller) V(level int) logr.InfoLogger {
	return c
}

func (c *Controller) WithValues(keysAndValues ...interface{}) logr.Logger {
	c.base.handler.Log(logrus.InfoLevel, keysAndValues...)
	return c
}

func (c *Controller) WithName(name string) logr.Logger {
	c.base.handler.WithField("name", name)
	return c
}
