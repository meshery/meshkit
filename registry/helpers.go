package registry

import (
	"io"
	"os"

	"github.com/layer5io/meshkit/logger"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// global logger variable
	Log logger.Handler
	//global logger error variable
	LogError logger.Handler
)

const (
	artifactHub            = "artifacthub"
	gitHub                 = "github"
	shouldRegisterColIndex = -1
	defaultURL             = "/pub?output=csv&gid="
	overrideURL            = "/export?format=csv&gid="
)

func GetIndexForRegisterCol(cols []string, shouldRegister string) int {
	if shouldRegisterColIndex != -1 {
		return shouldRegisterColIndex
	}

	for index, col := range cols {
		if col == shouldRegister {
			return index
		}
	}
	return shouldRegisterColIndex
}

// Initialize Meshkit Logger instance
func SetupLogger(name string, debugLevel bool, output io.Writer) logger.Handler {
	logLevel := viper.GetInt("LOG_LEVEL")
	if !debugLevel {
		logLevel = int(log.DebugLevel)
	}
	logger, err := logger.New(name, logger.Options{
		Format:   logger.TerminalLogFormat,
		LogLevel: logLevel,
		Output:   output,
	})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	return logger
}
