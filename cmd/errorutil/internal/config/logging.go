package config

import "github.com/sirupsen/logrus"

func Logging(verbose bool) {
	logrus.SetFormatter(&logrus.TextFormatter{})
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.WarnLevel)
	}
}
