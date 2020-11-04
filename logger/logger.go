package logger

import (
	"os"

	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/layer5io/meshkit/errors"
)

type Handler interface {
	Info(description ...string)
	Debug(description ...string)
	Warn(err error)
	Error(err error)
}

type Logger struct {
	handler kitlog.Logger
}

func New(appname string, opts Options) (Handler, error) {

	// Log Writers
	// errw := kitlog.NewSyncWriter(os.Stderr)
	infow := kitlog.NewSyncWriter(os.Stdout)
	logger := kitlog.NewLogfmtLogger(infow)

	// Formatter
	switch opts.Format {
	case JsonLogFormat:
		logger = kitlog.NewJSONLogger(infow)
	case SyslogLogFormat:
		logger = kitlog.NewLogfmtLogger(infow)
	}

	logger = level.NewFilter(logger, level.AllowAll())
	if !opts.DebugLevel {
		logger = level.NewFilter(logger, level.AllowInfo())
		logger = level.NewFilter(logger, level.AllowError())
		logger = level.NewFilter(logger, level.AllowWarn())
	}
	// Default fields
	logger = kitlog.WithPrefix(logger, "app", appname)
	logger = kitlog.WithPrefix(logger, "ts", kitlog.DefaultTimestamp)

	return &Logger{
		handler: logger,
	}, nil
}

func (l *Logger) Error(err error) {
	l.handler = kitlog.With(l.handler,
		"severity", errors.GetSeverity(err),
		"caller", kitlog.DefaultCaller,
		"code", errors.GetCode(err),
		"remedy", errors.GetRemedy(err),
	)
	_ = level.Error(l.handler).Log(err.Error())
}

func (l *Logger) Info(description ...string) {
	_ = level.Info(l.handler).Log(description)
}

func (l *Logger) Debug(description ...string) {
	_ = level.Debug(l.handler).Log(description)
}

func (l *Logger) Warn(err error) {
	l.handler = kitlog.With(l.handler,
		"severity", errors.GetSeverity(err),
		"caller", kitlog.DefaultCaller,
		"code", errors.GetCode(err),
		"remedy", errors.GetRemedy(err),
	)
	_ = level.Warn(l.handler).Log(err.Error())
}
