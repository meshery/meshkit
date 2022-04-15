package logger

import (
	"github.com/go-logr/logr"
	"github.com/layer5io/meshkit/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrControllerCode = "11071"
)

func ErrController(err error, msg string) error {
	return errors.New(ErrControllerCode, errors.Alert, []string{msg}, []string{err.Error()}, []string{}, []string{})
}

type Controller struct {
	enabled bool
	base    *Logger
}

func (l *Logger) ControllerLogger() logr.LogSink {
	return &Controller{
		enabled: true,
		base:    l,
	}
}

func (c *Controller) Init(info logr.RuntimeInfo) {}

func (c *Controller) Enabled(level int) bool {
	return c.enabled
}

func (c *Controller) Info(level int, msg string, keysAndValues ...interface{}) {
	c.base.Info(msg)
}

func (c *Controller) Error(err error, msg string, keysAndValues ...interface{}) {
	c.base.Error(ErrController(err, msg))
}

func (c *Controller) V(level int) *Controller {
	return c
}

func (c *Controller) WithValues(keysAndValues ...interface{}) logr.LogSink {
	c.base.handler.Log(logrus.InfoLevel, keysAndValues...)
	return c
}

func (c *Controller) WithName(name string) logr.LogSink {
	c.base.handler.WithField("name", name)
	return c
}
