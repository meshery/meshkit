package config

import "github.com/sirupsen/logrus"

func Logging(verbose bool) {
	logrus.SetFormatter(&logrus.TextFormatter{})
	//logrus.SetFormatter(&logrus.JSONFormatter{})
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
