package logger

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var logger *Logger

type Handler interface {
	Err(code string, des string)
	Debug(des string)
	Info(des string)
	EnableDebug(b bool)
}

type Logger struct {
	handler *logrus.Logger
	debug   bool
}

func New(appname string) (Handler, error) {

	log := logrus.New()

	log.SetFormatter(&logrus.JSONFormatter{})
	log.Out = os.Stdout

	host, _ := os.Hostname()
	log.WithFields(logrus.Fields{
		"host": host,
		"app":  appname,
		"ts":   time.Now().String(),
	})

	logger = &Logger{handler: log}

	return logger, nil
}

func Log(description string) {
	logger.Info(description)
}

func (l *Logger) EnableDebug(b bool) {
	l.debug = b
}

func (l *Logger) Err(code string, description string) {
	l.handler.WithFields(logrus.Fields{
		"code": code,
	}).Error(description)
}

func (l *Logger) Info(description string) {
	l.handler.Info(description)
}

func (l *Logger) Debug(description string) {
	if l.debug {
		l.handler.Debug(description)
	}
}
