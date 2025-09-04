package logger

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/meshery/meshkit/errors"
	"github.com/sirupsen/logrus"
	gormlogger "gorm.io/gorm/logger"
)

type Handler interface {
	Info(description ...interface{})
	Infof(format string, args ...interface{})
	Debug(description ...interface{})
	Debugf(format string, args ...interface{})
	Warn(err error)
	Warnf(format string, args ...interface{})
	Error(err error)
	SetLevel(level logrus.Level)
	GetLevel() logrus.Level
	UpdateLogOutput(w io.Writer)
	// Kubernetes Controller compliant logger
	ControllerLogger() logr.Logger
	DatabaseLogger() gormlogger.Interface
}

type Logger struct {
	handler *logrus.Entry
}

// TerminalFormatter is exported
type TerminalFormatter struct{}

// Format defined the format of output for Logrus logs
// Format is exported
func (f *TerminalFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var msg string
	if entry.HasCaller() {
		msg = fmt.Sprintf("[%s:%d %s] %s", entry.Caller.File, entry.Caller.Line, entry.Caller.Function, entry.Message)
	} else {
		msg = entry.Message
	}
	return append([]byte(msg), '\n'), nil
}

func New(appname string, opts Options) (Handler, error) {
	log := logrus.New()

	switch opts.Format {
	case JsonLogFormat:
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	case SyslogLogFormat:
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	case TerminalLogFormat:
		log.SetFormatter(new(TerminalFormatter))
	}

	log.SetReportCaller(opts.LogCallerInfo)
	log.SetOutput(os.Stdout)
	if opts.Output != nil {
		log.SetOutput(opts.Output)
	}

	log.SetLevel(logrus.Level(opts.LogLevel))

	entry := log.WithFields(logrus.Fields{"app": appname})
	return &Logger{handler: entry}, nil
}

func (l *Logger) Error(err error) {
	if err == nil {
		return
	}

	l.handler.WithFields(logrus.Fields{
		"code":                  errors.GetCode(err),
		"severity":              errors.GetSeverity(err),
		"short-description":     errors.GetSDescription(err),
		"probable-cause":        errors.GetCause(err),
		"suggested-remediation": errors.GetRemedy(err),
	}).Log(logrus.ErrorLevel, err.Error())
}

func (l *Logger) Info(description ...interface{}) {
	l.handler.Log(logrus.InfoLevel,
		description...,
	)
}

func (l *Logger) Debug(description ...interface{}) {
	l.handler.Log(logrus.DebugLevel,
		description...,
	)
}

func (l *Logger) Warn(err error) {
	if err == nil {
		return
	}

	l.handler.WithFields(logrus.Fields{
		"code":                  errors.GetCode(err),
		"severity":              errors.GetSeverity(err),
		"short-description":     errors.GetSDescription(err),
		"probable-cause":        errors.GetCause(err),
		"suggested-remediation": errors.GetRemedy(err),
	}).Log(logrus.WarnLevel, err.Error())
}

func (l *Logger) SetLevel(level logrus.Level) {
	l.handler.Logger.SetLevel(level)
}

func (l *Logger) GetLevel() logrus.Level {
	return l.handler.Logger.GetLevel()
}

func (l *Logger) UpdateLogOutput(output io.Writer) {
	l.handler.Logger.SetOutput(output)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.handler.Log(logrus.InfoLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.handler.Log(logrus.WarnLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.handler.Log(logrus.DebugLevel, fmt.Sprintf(format, args...))
}
