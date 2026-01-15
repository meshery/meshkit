package registry

import (
	"io"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshkit/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// global logger variable
	Log logger.Handler
	//global logger error variable
	LogError logger.Handler
	// logFile and errorLogFile are the file handles for the log files
	// They are stored at package level to allow proper cleanup
	logFile      *os.File
	errorLogFile *os.File
)

const (
	artifactHub                    = "artifacthub"
	gitHub                         = "github"
	shouldRegisterColIndex         = -1
	defaultURLPathAndQueryParams   = "/pub?output=csv&gid="
	overridedURLPathAndQueryParams = "/export?format=csv&gid="
	// DefaultModelTimeout is the default timeout for generating a single model (5 minutes)
	DefaultModelTimeout = 5 * time.Minute
)

// GenerationOptions contains configuration options for model generation
type GenerationOptions struct {
	// ModelTimeout is the timeout duration for generating a single model
	// Default is 5 minutes if not specified
	ModelTimeout time.Duration
	// LatestVersionOnly when true, generates only the latest version of each model
	LatestVersionOnly bool
	// ProgressCallback is called to report generation progress
	// Parameters: currentModel (name), currentIndex (1-based), totalModels
	ProgressCallback func(modelName string, currentIndex, totalModels int)
}

// DefaultGenerationOptions returns GenerationOptions with default values
func DefaultGenerationOptions() GenerationOptions {
	return GenerationOptions{
		ModelTimeout:      DefaultModelTimeout,
		LatestVersionOnly: false,
		ProgressCallback:  nil,
	}
}

// ProgressTracker tracks model generation progress in a thread-safe manner
type ProgressTracker struct {
	totalModels     int32
	processedModels int32
	successCount    int32
	failureCount    int32
	skippedCount    int32
}

// NewProgressTracker creates a new progress tracker with the given total
func NewProgressTracker(total int) *ProgressTracker {
	return &ProgressTracker{
		totalModels: int32(total),
	}
}

// Total returns the total number of models to process
func (pt *ProgressTracker) Total() int {
	return int(atomic.LoadInt32(&pt.totalModels))
}

// Processed returns the number of processed models
func (pt *ProgressTracker) Processed() int {
	return int(atomic.LoadInt32(&pt.processedModels))
}

// Remaining returns the number of remaining models to process
func (pt *ProgressTracker) Remaining() int {
	return pt.Total() - pt.Processed()
}

// IncrementProcessed increments the processed count and returns the new value
func (pt *ProgressTracker) IncrementProcessed() int {
	return int(atomic.AddInt32(&pt.processedModels, 1))
}

// IncrementSuccess increments the success count
func (pt *ProgressTracker) IncrementSuccess() {
	atomic.AddInt32(&pt.successCount, 1)
}

// IncrementFailure increments the failure count
func (pt *ProgressTracker) IncrementFailure() {
	atomic.AddInt32(&pt.failureCount, 1)
}

// IncrementSkipped increments the skipped count
func (pt *ProgressTracker) IncrementSkipped() {
	atomic.AddInt32(&pt.skippedCount, 1)
}

// SuccessCount returns the number of successfully generated models
func (pt *ProgressTracker) SuccessCount() int {
	return int(atomic.LoadInt32(&pt.successCount))
}

// FailureCount returns the number of failed model generations
func (pt *ProgressTracker) FailureCount() int {
	return int(atomic.LoadInt32(&pt.failureCount))
}

// SkippedCount returns the number of skipped models
func (pt *ProgressTracker) SkippedCount() int {
	return int(atomic.LoadInt32(&pt.skippedCount))
}

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

// CloseLogger closes the log file handles opened by SetLogger.
// This should be called when logging is no longer needed to prevent file handle leaks.
func CloseLogger() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	if errorLogFile != nil {
		_ = errorLogFile.Close()
		errorLogFile = nil
	}
}

// Downloads CSV file using spreadsheet URL
func DownloadCSVAndGetDownloadURL(sheetURL ,csvPath string, spreadsheetID int64)  (string ,error){
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