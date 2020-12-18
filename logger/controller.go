package logger

import (
	kitlog "github.com/go-kit/kit/log"
	"github.com/go-logr/logr"
)

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
	c.base.Error(err)
}

func (c *Controller) V(level int) logr.InfoLogger {
	return c
}

func (c *Controller) WithValues(keysAndValues ...interface{}) logr.Logger {
	c.base.infoHandler = kitlog.With(c.base.infoHandler, keysAndValues)
	c.base.errHandler = kitlog.With(c.base.errHandler, keysAndValues)
	return c
}

func (c *Controller) WithName(name string) logr.Logger {
	c.base.infoHandler = kitlog.With(c.base.infoHandler, "name", name)
	c.base.errHandler = kitlog.With(c.base.errHandler, "name", name)
	return c
}
