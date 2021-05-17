package main

import (
	"os"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/coder"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})

	rootCmd := coder.RootCommand()
	err := rootCmd.Execute()
	if err != nil {
		log.Errorf("Unable to execute root command (%v)", err)
		return
	}
}
