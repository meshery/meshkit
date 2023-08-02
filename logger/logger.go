package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/layer5io/meshkit/errors"
	"github.com/sirupsen/logrus"
	gormlogger "gorm.io/gorm/logger"
)

type Handler interface {
	SetLogFormatter(formatter TerminalFormatter)

	Info(description ...interface{})
	Debug(description ...interface{})
	Warn(err error)
	Error(err error)

	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})

	// Kubernetes Controller compliant logger
	ControllerLogger() logr.Logger
	DatabaseLogger() gormlogger.Interface
}

type Logger struct {
	handler      *logrus.Entry
	logFormatter TerminalFormatter
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
		log.SetFormatter(&TerminalFormatter{})
	}

	// log.SetReportCaller(true)
	log.SetOutput(os.Stdout)
	if opts.Output != nil {
		log.SetOutput(opts.Output)
	}

	log.SetLevel(logrus.InfoLevel)
	if opts.DebugLevel {
		log.SetLevel(logrus.DebugLevel)
	}

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

func (l *Logger) SetLogFormatter(formatter TerminalFormatter) {
	l.logFormatter = formatter
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.handler.Logger.Error(message)
}

func (l *Logger) Info(description ...interface{}) {
	l.handler.Log(logrus.InfoLevel,
		description...,
	)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.handler.Log(logrus.InfoLevel, message)
}

func (l *Logger) Debug(description ...interface{}) {
	l.handler.Log(logrus.DebugLevel,
		description...,
	)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.handler.Log(logrus.DebugLevel, message)
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

func (l *Logger) Warnf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.handler.Log(logrus.WarnLevel, message)
}
