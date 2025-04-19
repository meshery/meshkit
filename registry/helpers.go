package registry

import (
	"io"
	"os"
	"strconv"

	"github.com/layer5io/meshkit/logger"
	"github.com/layer5io/meshkit/utils"
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
	artifactHub                    = "artifacthub"
	gitHub                         = "github"
	shouldRegisterColIndex         = -1
	defaultURLPathAndQueryParams   = "/pub?output=csv&gid="
	overridedURLPathAndQueryParams = "/export?format=csv&gid="
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

// Downloads CSV file using spreadsheet URL
func DownloadCSV(sheetURL ,csvPath string, spreadsheetID int64)  (string ,error){
	newSheetURL := sheetURL + defaultURLPathAndQueryParams + strconv.FormatInt(spreadsheetID, 10)
	err := utils.DownloadFile(csvPath, newSheetURL)

	// The `/pub?output=csv` URL format has been reported to fail for some users, possibly due to a bug or deprecation.
	// As a workaround, we use an alternative URL format `/export?format=csv`, which works reliably in such cases.
	// To maintain backward compatibility, the alternative format (overridedURLPathAndQueryParams) is only used
	// if the default format (defaultURLPathAndQueryParams) fails.
	// Additionally, the `/export?format=csv` format has the advantage of not requiring the sheet to be explicitly
	// published to the web, unlike the default format which requires the sheet to be published in CSV format.
	
	if err !=nil{
		newSheetURL = sheetURL + overridedURLPathAndQueryParams + strconv.FormatInt(spreadsheetID, 10)
		err = utils.DownloadFile(csvPath, newSheetURL)
		if err !=nil{
			return "",utils.ErrReadingRemoteFile(err)
		}
	}
	Log.Info("Downloaded CSV from: ", newSheetURL)
	return newSheetURL,nil
}