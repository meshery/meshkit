package logger

import "github.com/sirupsen/logrus"

const (
	JsonLogFormat = iota
	SyslogLogFormat
	TerminalLogFormat
)

type Format int

// TerminalFormatter is exported
type TerminalFormatter struct{}

// Format defined the format of output for Logrus logs
// Format is exported
func (f *TerminalFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}
