package logger

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	gormlogger "gorm.io/gorm/logger"
)

type Database struct {
	enabled bool
	base    *Logger
}

func (l *Logger) DatabaseLogger() gormlogger.Interface {
	return &Database{
		enabled: true,
		base:    l,
	}
}

func (c *Database) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return c
}
func (c *Database) Info(ctx context.Context, msg string, data ...interface{}) {
	c.base.handler.Log(logrus.InfoLevel,
		"msg", data,
	)
}
func (c *Database) Warn(ctx context.Context, msg string, data ...interface{}) {
	c.base.handler.Log(logrus.WarnLevel,
		"msg", data,
	)
}
func (c *Database) Error(ctx context.Context, msg string, data ...interface{}) {
	c.base.handler.Log(logrus.ErrorLevel,
		"msg", data,
	)
}
func (c *Database) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
}
