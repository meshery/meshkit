package logger

import "io"

type Options struct {
	Format     Format
	DebugLevel bool
	Output     io.Writer
}
