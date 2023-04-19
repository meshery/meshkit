package logger

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/layer5io/meshkit/errors"
	"golang.org/x/exp/slog"
)

const callerStackDepth = 1
const callerSkip = 2

var (
	ErrControllerCode = "11071"
)

func ErrController(err error, msg string) error {
	return errors.New(ErrControllerCode, errors.Alert, []string{msg}, []string{err.Error()}, []string{}, []string{})
}

func (cl *Logger) ControllerLogger() slog.Logger {
	return *slog.New(cl.Handler)
}

func (cl *Controller) Init(info context.Context) {}

func (cl *Controller) Enabled(ctx context.Context, level slog.Level) bool {
	return cl.Handler.Enabled(ctx, level)
}

func (cl *Controller) Info(msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelInfo, msg, pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Warn(msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelWarn) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelWarn, msg, pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Error(msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelError) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelError, msg, pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Debug(msg string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelDebug, msg, pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Infof(format string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Errorf(format string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelError) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelError, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Warnf(format string, args ...interface{}) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelWarn) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelWarn, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) Debugf(format string, args ...interface{}) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelDebug, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

func (cl *Controller) V(level int) *Controller {
	return cl
}

func (cl *Controller) WithAttrs(ctx context.Context, attrs []slog.Attr) slog.Handler {
	return nil
	// return &Logger{
	//	Handler: cl.Handler.WithAttrs(attrs),
	//}
}

func (cl *Controller) WithGroup(ctx context.Context, name string) slog.Handler {
	return nil
	// return &Logger{
	//	Handler: cl.Handler.WithGroup(name),
	//}
}
