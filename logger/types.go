package logger

const (
	JsonLogFormat = iota
	SyslogLogFormat
)

type Format int

type Options struct {
	Format     Format
	DebugLevel bool
}
