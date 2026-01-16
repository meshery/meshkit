package registry

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultGenerationOptions(t *testing.T) {
	opts := DefaultGenerationOptions()

	assert.Equal(t, DefaultModelTimeout, opts.ModelTimeout, "ModelTimeout should be DefaultModelTimeout (5 minutes)")
	assert.False(t, opts.LatestVersionOnly, "LatestVersionOnly should be false by default")
	assert.Nil(t, opts.ProgressCallback, "ProgressCallback should be nil by default")
}

func TestGenerationOptionsWithCustomValues(t *testing.T) {
	customTimeout := 10 * time.Minute
	callbackCalled := false

	opts := GenerationOptions{
		ModelTimeout:      customTimeout,
		LatestVersionOnly: true,
		ProgressCallback: func(modelName string, currentIndex, totalModels int) {
			callbackCalled = true
		},
	}

	assert.Equal(t, customTimeout, opts.ModelTimeout)
	assert.True(t, opts.LatestVersionOnly)
	assert.NotNil(t, opts.ProgressCallback)

	// Test callback can be invoked
	opts.ProgressCallback("test-model", 1, 10)
	assert.True(t, callbackCalled, "ProgressCallback should have been called")
}

func TestProgressTrackerCreation(t *testing.T) {
	tracker := NewProgressTracker(100)

	assert.Equal(t, 100, tracker.Total(), "Total should be 100")
	assert.Equal(t, 0, tracker.Processed(), "Processed should be 0 initially")
	assert.Equal(t, 100, tracker.Remaining(), "Remaining should be 100 initially")
	assert.Equal(t, 0, tracker.SuccessCount(), "SuccessCount should be 0 initially")
	assert.Equal(t, 0, tracker.FailureCount(), "FailureCount should be 0 initially")
	assert.Equal(t, 0, tracker.SkippedCount(), "SkippedCount should be 0 initially")
}

func TestProgressTrackerIncrements(t *testing.T) {
	tracker := NewProgressTracker(10)

	// Test IncrementProcessed
	newCount := tracker.IncrementProcessed()
	assert.Equal(t, 1, newCount, "IncrementProcessed should return 1")
	assert.Equal(t, 1, tracker.Processed(), "Processed should be 1")
	assert.Equal(t, 9, tracker.Remaining(), "Remaining should be 9")

	// Test IncrementSuccess
	tracker.IncrementSuccess()
	assert.Equal(t, 1, tracker.SuccessCount(), "SuccessCount should be 1")

	// Test IncrementFailure
	tracker.IncrementFailure()
	assert.Equal(t, 1, tracker.FailureCount(), "FailureCount should be 1")

	// Test IncrementSkipped
	tracker.IncrementSkipped()
	assert.Equal(t, 1, tracker.SkippedCount(), "SkippedCount should be 1")
}

func TestProgressTrackerConcurrency(t *testing.T) {
	tracker := NewProgressTracker(1000)
	var wg sync.WaitGroup

	// Simulate concurrent increments
	for i := 0; i < 100; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			tracker.IncrementProcessed()
		}()
		go func() {
			defer wg.Done()
			tracker.IncrementSuccess()
		}()
		go func() {
			defer wg.Done()
			tracker.IncrementFailure()
		}()
		go func() {
			defer wg.Done()
			tracker.IncrementSkipped()
		}()
	}

	wg.Wait()

	assert.Equal(t, 100, tracker.Processed(), "Processed should be 100 after concurrent increments")
	assert.Equal(t, 100, tracker.SuccessCount(), "SuccessCount should be 100 after concurrent increments")
	assert.Equal(t, 100, tracker.FailureCount(), "FailureCount should be 100 after concurrent increments")
	assert.Equal(t, 100, tracker.SkippedCount(), "SkippedCount should be 100 after concurrent increments")
	assert.Equal(t, 900, tracker.Remaining(), "Remaining should be 900")
}

func TestProgressTrackerZeroTotal(t *testing.T) {
	tracker := NewProgressTracker(0)

	assert.Equal(t, 0, tracker.Total(), "Total should be 0")
	assert.Equal(t, 0, tracker.Remaining(), "Remaining should be 0")

	// Incrementing should still work
	tracker.IncrementProcessed()
	assert.Equal(t, 1, tracker.Processed(), "Processed should be 1")
	assert.Equal(t, -1, tracker.Remaining(), "Remaining should be -1 (underflow is acceptable)")
}

func TestDefaultModelTimeoutValue(t *testing.T) {
	expectedTimeout := 5 * time.Minute
	assert.Equal(t, expectedTimeout, DefaultModelTimeout, "DefaultModelTimeout should be 5 minutes")
}

func TestProgressCallbackParameters(t *testing.T) {
	var capturedModelName string
	var capturedCurrentIndex int
	var capturedTotalModels int

	opts := GenerationOptions{
		ProgressCallback: func(modelName string, currentIndex, totalModels int) {
			capturedModelName = modelName
			capturedCurrentIndex = currentIndex
			capturedTotalModels = totalModels
		},
	}

	opts.ProgressCallback("kubernetes", 5, 100)

	assert.Equal(t, "kubernetes", capturedModelName)
	assert.Equal(t, 5, capturedCurrentIndex)
	assert.Equal(t, 100, capturedTotalModels)
}

func TestCloseLoggerWhenNoFilesOpen(t *testing.T) {
	// Ensure logFile and errorLogFile are nil
	logFile = nil
	errorLogFile = nil

	// CloseLogger should not panic when files are nil
	assert.NotPanics(t, func() {
		CloseLogger()
	}, "CloseLogger should not panic when log files are nil")

	// After closing, files should still be nil
	assert.Nil(t, logFile, "logFile should be nil after CloseLogger")
	assert.Nil(t, errorLogFile, "errorLogFile should be nil after CloseLogger")
}
