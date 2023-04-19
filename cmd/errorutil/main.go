package main

import (
	"os"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/coder"
	meshlogger "github.com/layer5io/meshkit/cmd/errorutil/logger"
	"golang.org/x/exp/slog"
)

var logger = slog.New(slog.HandlerOptions{}.NewJSONHandler(os.Stdout))

func main() {
	h := slog.HandlerOptions{Level: slog.LevelInfo}.NewJSONHandler(os.Stderr)
	slog.SetDefault(slog.New(h))
	rootCmd := coder.RootCommand()

	err := rootCmd.Execute()
	if err != nil {
		meshlogger.Errorf(logger, "Unable to execute root command (%v)", err)
		return
	}
}
