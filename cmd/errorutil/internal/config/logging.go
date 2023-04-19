package config

import (
	"os"

	"golang.org/x/exp/slog"
)

var programLevel = new(slog.LevelVar)

func Logger(verbose bool) {
	h := slog.HandlerOptions{Level: programLevel}.NewJSONHandler(os.Stderr)
	slog.SetDefault(slog.New(h))

	if verbose {
		programLevel.Set(slog.LevelDebug)
	} else {
		programLevel.Set(slog.LevelWarn)
	}
}
