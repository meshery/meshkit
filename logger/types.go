package logger

const (
	JsonLogFormat = iota
	SyslogLogFormat
	TerminalLogFormat
)

type Format int

type Options struct {
	Format     Format
	DebugLevel bool
}
