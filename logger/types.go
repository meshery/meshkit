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
	Format   Format
	LogLevel int
	Output   io.Writer
}
