package main

import (
	"os"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/coder"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	mesherr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
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

	componentInfo, err := component.ReadComponentInfoFile(".") // TODO set dir to rootDir!
	if err != nil {
		log.Fatalf("Unable to read component info file (%v)", err)
		return
	}

	var errorsInfo = mesherr.NewErrorsInfo()

	rootCmd := coder.RootCommand(errorsInfo)
	err = rootCmd.Execute()
	if err != nil {
		log.Errorf("Unable to execute root command (%v)", err)
		return
	}
	mesherr.Summarize(errorsInfo)
	mesherr.Export(componentInfo, errorsInfo)
}
