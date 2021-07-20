package logger

const (
	JSONLogFormat = iota
	SyslogLogFormat
)

type Format int

type Options struct {
	Format     Format
	DebugLevel bool
}
