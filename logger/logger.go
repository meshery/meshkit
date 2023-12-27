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
	Info(description ...interface{})
	Debug(description ...interface{})
	Warn(err error)
	Error(err error)

	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Errorf(err error, format string, args ...interface{})
	Warnf(err error, format string, args ...interface{})

	// Kubernetes Controller compliant logger
	ControllerLogger() logr.Logger
	DatabaseLogger() gormlogger.Interface

	SetLevel(level logrus.Level)
}

type Logger struct {
	handler *logrus.Entry
}

func (l *Logger) SetLevel(level logrus.Level) {
	l.handler.Logger.SetLevel(level)
}

// TerminalFormatter is exported
type TerminalFormatter struct{}

// Format defined the format of output for Logrus logs
func (f *TerminalFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if entry.Level == logrus.ErrorLevel || entry.Level == logrus.WarnLevel {
		return f.formatDetails(entry), nil
	}
	return append([]byte(entry.Message), '\n'), nil
}

func (f *TerminalFormatter) formatDetails(entry *logrus.Entry) []byte {
	level := "Error"
	if entry.Level == logrus.WarnLevel {
		level = "Warning"
	}

	output := fmt.Sprintf("%s: %s\n", level, entry.Message)
	output += fmt.Sprintf("  Code:                  %v\n", entry.Data["code"])
	output += fmt.Sprintf("  Severity:              %v\n", entry.Data["severity"])
	output += fmt.Sprintf("  Short Description:     %v\n", entry.Data["short-description"])
	output += fmt.Sprintf("  Probable Cause:        %v\n", entry.Data["probable-cause"])
	output += fmt.Sprintf("  Suggested Remediation: %v\n", entry.Data["suggested-remediation"])

	return []byte(output)
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

func (l *Logger) Errorf(err error, format string, args ...interface{}) {
	if err == nil {
		return
	}

	message := fmt.Sprintf(format, args...)
	fullMessage := fmt.Sprintf("%s: %s", message, err.Error())

	l.handler.WithFields(logrus.Fields{
		"code":                  errors.GetCode(err),
		"severity":              errors.GetSeverity(err),
		"short-description":     errors.GetSDescription(err),
		"probable-cause":        errors.GetCause(err),
		"suggested-remediation": errors.GetRemedy(err),
	}).Log(logrus.ErrorLevel, fullMessage)
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

func (l *Logger) Warnf(err error, format string, args ...interface{}) {
	if err == nil {
		return
	}

	message := fmt.Sprintf(format, args...)
	fullMessage := fmt.Sprintf("%s: %s", message, err.Error())

	l.handler.WithFields(logrus.Fields{
		"code":                  errors.GetCode(err),
		"severity":              errors.GetSeverity(err),
		"short-description":     errors.GetSDescription(err),
		"probable-cause":        errors.GetCause(err),
		"suggested-remediation": errors.GetRemedy(err),
	}).Log(logrus.WarnLevel, fullMessage)
}
