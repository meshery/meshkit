package main

import (
	"os"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/coder"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})

	rootCmd := coder.RootCommand()
	err = rootCmd.Execute()
	if err != nil {
		log.Errorf("Unable to execute root command (%v)", err)
		return
	}
}
