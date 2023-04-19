package logger

import (
	"context"
	"time"

	"golang.org/x/exp/slog"
	gormlogger "gorm.io/gorm/logger"
)

type Database struct {
	Logger    slog.Handler
	gormLevel gormlogger.LogLevel
	enabled   bool
}

func (dl *Logger) DatabaseLogger() gormlogger.Interface {
	return &Database{
		Logger:    dl.Handler,
		gormLevel: gormlogger.LogLevel(0),
		enabled:   true,
	}
}

// Error implements gormlogger.Interface.
func (dl *Database) Error(ctx context.Context, msg string, data ...interface{}) {
	if dl.gormLevel >= gormlogger.Error {
		slog.ErrorCtx(ctx, msg, data...)
	}
}

// Info implements gormlogger.Interface.
func (dl *Database) Info(ctx context.Context, msg string, data ...interface{}) {
	if dl.gormLevel >= gormlogger.Info {
		slog.InfoCtx(ctx, msg, data...)
	}
}

// Trace implements gormlogger.Interface.
func (dl *Database) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if dl.gormLevel >= gormlogger.Info {
		elapsed := time.Since(begin)
		sql, rows := fc()
		slog.InfoCtx(ctx, sql, slog.Int64("rows", rows), slog.Duration("elapsed", elapsed))
	}
}

// Warn implements gormlogger.Interface.
func (dl *Database) Warn(ctx context.Context, msg string, data ...interface{}) {
	if dl.gormLevel >= gormlogger.Warn {
		slog.WarnCtx(ctx, msg, data...)
	}
}

type DatabaseEntry struct {
	Logger *slog.Logger
}

// LogMode implements gormlogger.Interface.
func (dl *Database) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return &Database{
		Logger:    dl.Logger,
		enabled:   true,
		gormLevel: gormlogger.LogLevel(0),
	}
	// newLogger := *dl
	// newLogger.gormLevel = level
	// return &newLogger
}
