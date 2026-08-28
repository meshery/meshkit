package logger

import (
	"context"
	"fmt"
	"strconv"
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
	c.base.defaultHandler.Log(logrus.InfoLevel,
		msg, data,
	)
}
func (c *Database) Warn(ctx context.Context, msg string, data ...interface{}) {
	c.base.defaultHandler.Log(logrus.WarnLevel,
		msg, data,
	)
}
func (c *Database) Error(ctx context.Context, msg string, data ...interface{}) {
	c.base.errorHandler.Log(logrus.ErrorLevel,
		msg, data,
	)
}

// Trace is called by GORM after every SQL statement. It forwards the statement, its execution
// time, the affected row count and any error to the underlying logger, matching the format used
// by GORM's own logger.
func (c *Database) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if c == nil || c.base == nil || fc == nil {
		return
	}
	sql, rows := fc()
	elapsed := float64(time.Since(begin).Nanoseconds()) / 1e6

	// GORM reports rows as -1 when a row count does not apply to the statement.
	affected := strconv.FormatInt(rows, 10)
	if rows == -1 {
		affected = "-"
	}

	if err != nil {
		c.base.errorHandler.Log(logrus.ErrorLevel,
			fmt.Sprintf("%v [%.3fms] [rows:%s] %s", err, elapsed, affected, sql),
		)
		return
	}
	c.base.defaultHandler.Log(logrus.InfoLevel,
		fmt.Sprintf("[%.3fms] [rows:%s] %s", elapsed, affected, sql),
	)
}
