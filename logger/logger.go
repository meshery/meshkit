package logger

import (
	"os"
	"strings"

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
	infoHandler kitlog.Logger
	errHandler  kitlog.Logger
}

func New(appname string, opts Options) (Handler, error) {

	// Log Writers
	infow := kitlog.NewSyncWriter(os.Stdout)
	errw := kitlog.NewSyncWriter(os.Stderr)
	infoLogger := kitlog.NewLogfmtLogger(infow)
	errLogger := kitlog.NewLogfmtLogger(errw)

	// Formatter
	switch opts.Format {
	case JsonLogFormat:
		infoLogger = kitlog.NewJSONLogger(infow)
		errLogger = kitlog.NewJSONLogger(errw)
	case SyslogLogFormat:
		infoLogger = kitlog.NewLogfmtLogger(infow)
		errLogger = kitlog.NewLogfmtLogger(errw)
	}

	// Default fields
	infoLogger = kitlog.WithPrefix(infoLogger, "app", appname, "ts", kitlog.DefaultTimestamp)
	infoLogger = level.NewFilter(infoLogger, level.AllowAll())
	if !opts.DebugLevel {
		infoLogger = level.NewFilter(infoLogger, level.AllowInfo())
	}

	errLogger = kitlog.WithPrefix(errLogger,
		"app", appname,
		"ts", kitlog.DefaultTimestamp,
		"caller", kitlog.DefaultCaller,
	)
	errLogger = level.NewFilter(errLogger, level.AllowError())
	errLogger = level.NewFilter(errLogger, level.AllowWarn())

	return &Logger{
		infoHandler: infoLogger,
		errHandler:  errLogger,
	}, nil
}

func (l *Logger) Error(err error) {
	l.errHandler = kitlog.With(l.errHandler,
		"code", errors.GetCode(err),
		"severity", errors.GetSeverity(err),
		"short-description", errors.GetSDescription(err),
		"long-description", err.Error(),
		"probable-cause", errors.GetCause(err),
		"suggested-remediation", errors.GetRemedy(err),
	)

	er := level.Error(l.errHandler).Log()
	if er != nil {
		_ = l.errHandler.Log("Internal Logger Error")
	}
}

func (l *Logger) Info(description ...string) {
	err := level.Info(l.infoHandler).Log("message", strings.Join(description, ""))
	if err != nil {
		_ = l.errHandler.Log("Internal Logger Error")
	}
}

func (l *Logger) Debug(description ...string) {
	err := level.Debug(l.infoHandler).Log("message", strings.Join(description, ""))
	if err != nil {
		_ = l.errHandler.Log("Internal Logger Error")
	}
}

func (l *Logger) Warn(err error) {
	l.errHandler = kitlog.With(l.errHandler,
		"code", errors.GetCode(err),
		"severity", errors.GetSeverity(err),
		"short-description", errors.GetSDescription(err),
		"long-description", err.Error(),
		"probable-cause", errors.GetCause(err),
		"suggested-remediation", errors.GetRemedy(err),
	)

	er := level.Warn(l.errHandler).Log()
	if er != nil {
		_ = l.errHandler.Log("Internal Logger Error")
	}
}
