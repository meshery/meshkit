package logger

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/layer5io/meshkit/errors"
	"golang.org/x/exp/slog"
)

func (h *Logger) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.Handler.Enabled(ctx, lvl)
}

func (H *Logger) Handle(ctx context.Context, rec slog.Record) error {
	return H.Handler.Handle(ctx, rec)
}

func (H *Logger) LogAttrs(ctx context.Context, lvl slog.Level, msg string, attrs ...slog.Attr) {
	H.Logger.LogAttrs(ctx, lvl, msg, attrs...)
}

func (h *Logger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.Handler.WithAttrs(attrs)
}

func (h *Logger) WithGroup(name string) slog.Handler {
	return h.Handler.WithGroup(name)
}

// -----------------------------------------------------------------
// Info, Infof
// -----------------------------------------------------------------
func (h *Logger) Infof(format string, args ...any) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}

// -----------------------------------------------------------------
// Debug, Debugf
// -----------------------------------------------------------------

func (h *Logger) Debugf(format string, args ...interface{}) {
	l := slog.Default()
	if !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	var pcs [callerStackDepth]uintptr
	runtime.Callers(callerSkip, pcs[:])
	r := slog.NewRecord(
		time.Now(),
		slog.LevelDebug,
		fmt.Sprintf(format, args...),
		pcs[0],
	)
	_ = l.Handler().Handle(context.Background(), r)
}

// -----------------------------------------------------------------
// Warn, Warnf
// -----------------------------------------------------------------
func (h *Logger) Warn(msg string, err error, args ...any) {
	// func (h *Logger) Warn(err error) {
	if err == nil {
		h.LogAttrs(
			context.Background(),
			slog.LevelWarn,
			"logging warning...",
			slog.String("code", errors.GetCode(err)),
			slog.String("severity", fmt.Sprint(errors.GetSeverity(err))),
			slog.String("short-description", errors.GetSDescription(err)),
			slog.String("probable-cause", errors.GetCause(err)),
			slog.String("suggested-remediation", errors.GetRemedy(err)),
		)
	}
}

func (h *Logger) Warnf(format string, args ...interface{}) {
	// l := slog.Default()
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	r := slog.NewRecord(
		time.Now(),
		slog.LevelWarn,
		fmt.Sprintf(format, args...),
		pcs[0],
	)
	_ = h.Handler.Handle(context.Background(), r)
}

// -----------------------------------------------------------------
// Error, Errorf
// -----------------------------------------------------------------
func (h *Logger) Error(msg string, err error, args ...any) {
	// func (h *Logger) Error(err error) {
	if err == nil {
		return
	}

	h.LogAttrs(
		context.Background(),
		slog.LevelError,
		"error trace",
		slog.String("code", errors.GetCode(err)),
		slog.String("severity", fmt.Sprint(errors.GetSeverity(err))),
		slog.String("short-description", errors.GetSDescription(err)),
		slog.String("probable-cause", errors.GetCause(err)),
		slog.String("suggested-remediation", errors.GetRemedy(err)),
	)
}

func (h *Logger) Errorf(format string, args ...interface{}) {
	// l := slog.Default()
	if !h.Enabled(context.Background(), slog.LevelError) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	r := slog.NewRecord(
		time.Now(),
		slog.LevelError,
		fmt.Sprintf(format, args...),
		pcs[0],
	)
	_ = h.Handler.Handle(context.Background(), r)
}

// -----------------------------------------------------------------
// Define Logger
// -----------------------------------------------------------------
func createHandler(formatOptions Options, lvl *slog.LevelVar) slog.Handler {
	slogOpts := slog.HandlerOptions{
		Level:     lvl,
		AddSource: false,
	}

	switch formatOptions.Format {
	case JSONLogFormat:
		return slogOpts.NewJSONHandler(os.Stdout)
	case TextLogFormat:
		return slogOpts.NewTextHandler(os.Stdout)
	default:
		return slogOpts.NewTextHandler(os.Stdout)
	}
}

func New(appName string, formatOptions Options) (Handler, error) {
	levelVar := new(slog.LevelVar)
	handler := createHandler(formatOptions, levelVar)
	if levelVar != nil {
		levelVar.Set(slog.LevelDebug)
	} else {
		levelVar.Set(slog.LevelInfo)
	}

	logger := slog.New(handler)

	instanceID := generateID()

	buildInfo, _ := debug.ReadBuildInfo()

	logger.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"program_info",
		slog.String("app_name", appName),
		slog.Int("id", instanceID),
		slog.Group("properties", slog.Int("pid", os.Getpid()), slog.String("go_version", buildInfo.GoVersion)),
	)

	return &Logger{
		Handler: handler,
		Logger:  logger,
	}, nil
}

func generateID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Int()
}
