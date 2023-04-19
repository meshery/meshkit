package logger

import (
	"context"
	"io"

	"golang.org/x/exp/slog"
	gormlogger "gorm.io/gorm/logger"
	"k8s.io/client-go/kubernetes"
)

const (
	JSONLogFormat = iota
	TextLogFormat
	PrettyJSONLogFormat
)

type Controller struct {
	slog.Handler
	logger  *slog.Logger
	enabled bool
	client  kubernetes.Interface
}

type Format int

type Options struct {
	Format     Format
	DebugLevel bool
	Output     io.Writer
}

type Handler interface {
	Enabled(ctx context.Context, lvl slog.Level) bool
	Handle(ctx context.Context, rec slog.Record) error
	WithAttrs(attrs []slog.Attr) slog.Handler
	WithGroup(name string) slog.Handler

	Debug(msg string, args ...any)
	DebugCtx(ctx context.Context, msg string, args ...any)
	Error(msg string, err error, args ...any)
	// Error(err error)
	ErrorCtx(ctx context.Context, msg string, args ...any)
	Info(msg string, args ...any)
	InfoCtx(ctx context.Context, msg string, args ...any)
	Warn(msg string, err error, args ...any)
	// Warn(err error)
	WarnCtx(ctx context.Context, msg string, args ...any)

	ControllerLogger() slog.Logger
	DatabaseLogger() gormlogger.Interface
}

type Logger struct {
	slog.Handler
	*slog.Logger
}
