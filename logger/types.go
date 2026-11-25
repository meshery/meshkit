package logger

import (
	"io"
)

const (
	JsonLogFormat = iota
	SyslogLogFormat
	TerminalLogFormat
)

type Format int

type Options struct {
	Format             Format
	LogLevel           int
	Output             io.Writer // Non-error logs. Defaults to os.Stdout.
	ErrorOutput        io.Writer // Error and Fatal logs. Defaults to Output, then os.Stderr.
	EnableCallerInfo   bool
	CallerSkippedPaths []string
}
