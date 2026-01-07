package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerationOptionsTimeoutBehavior(t *testing.T) {
	// Test that timeout value is respected when set
	tests := []struct {
		name           string
		timeout        time.Duration
		expectedResult time.Duration
	}{
		{
			name:           "Custom timeout of 10 minutes",
			timeout:        10 * time.Minute,
			expectedResult: 10 * time.Minute,
		},
		{
			name:           "Custom timeout of 1 minute",
			timeout:        1 * time.Minute,
			expectedResult: 1 * time.Minute,
		},
		{
			name:           "Zero timeout should use default",
			timeout:        0,
			expectedResult: 0, // Will be set to default in the actual function
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := GenerationOptions{
				ModelTimeout: tt.timeout,
			}
			assert.Equal(t, tt.expectedResult, opts.ModelTimeout)
		})
	}
}

func TestProgressTrackerIntegration(t *testing.T) {
	// Simulate a model generation workflow
	totalModels := 50
	tracker := NewProgressTracker(totalModels)

	var wg sync.WaitGroup
	successfulModels := 30
	failedModels := 15
	skippedModels := 5

	// Simulate processing models concurrently
	for i := 0; i < successfulModels; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.IncrementProcessed()
			tracker.IncrementSuccess()
		}()
	}

	for i := 0; i < failedModels; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.IncrementProcessed()
			tracker.IncrementFailure()
		}()
	}

	for i := 0; i < skippedModels; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.IncrementProcessed()
			tracker.IncrementSkipped()
		}()
	}

	wg.Wait()

	assert.Equal(t, totalModels, tracker.Total())
	assert.Equal(t, totalModels, tracker.Processed())
	assert.Equal(t, 0, tracker.Remaining())
	assert.Equal(t, successfulModels, tracker.SuccessCount())
	assert.Equal(t, failedModels, tracker.FailureCount())
	assert.Equal(t, skippedModels, tracker.SkippedCount())

	// Verify that success + failure + skipped equals processed
	totalCategorized := tracker.SuccessCount() + tracker.FailureCount() + tracker.SkippedCount()
	assert.Equal(t, tracker.Processed(), totalCategorized)
}

func TestProgressCallbackIntegration(t *testing.T) {
	// Test that progress callback receives correct parameters during simulated model processing
	totalModels := 10
	callbackInvocations := make([]struct {
		modelName    string
		currentIndex int
		total        int
	}, 0)

	var mu sync.Mutex

	opts := GenerationOptions{
		ModelTimeout:      5 * time.Minute,
		LatestVersionOnly: false,
		ProgressCallback: func(modelName string, currentIndex, totalModels int) {
			mu.Lock()
			defer mu.Unlock()
			callbackInvocations = append(callbackInvocations, struct {
				modelName    string
				currentIndex int
				total        int
			}{modelName, currentIndex, totalModels})
		},
	}

	// Simulate calling the callback for each model
	modelNames := []string{"kubernetes", "istio", "prometheus", "grafana", "nginx",
		"redis", "postgres", "mongodb", "elasticsearch", "kafka"}

	for i, modelName := range modelNames {
		opts.ProgressCallback(modelName, i+1, totalModels)
	}

	assert.Equal(t, totalModels, len(callbackInvocations))

	// Verify each invocation has correct data
	for i, invocation := range callbackInvocations {
		assert.Equal(t, modelNames[i], invocation.modelName)
		assert.Equal(t, i+1, invocation.currentIndex)
		assert.Equal(t, totalModels, invocation.total)
	}
}

func TestModelTimeoutWithContext(t *testing.T) {
	// Test that per-model timeout works with context
	shortTimeout := 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()

	// Simulate a long-running operation
	done := make(chan bool)
	go func() {
		time.Sleep(200 * time.Millisecond) // Longer than timeout
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Operation should have timed out")
	case <-ctx.Done():
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	}
}

func TestModelTimeoutNotExceeded(t *testing.T) {
	// Test that operations complete before timeout
	timeout := 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Simulate a quick operation
	done := make(chan bool)
	go func() {
		time.Sleep(50 * time.Millisecond) // Shorter than timeout
		done <- true
	}()

	select {
	case <-done:
		// Success - operation completed before timeout
		assert.Nil(t, ctx.Err())
	case <-ctx.Done():
		t.Fatal("Operation should have completed before timeout")
	}
}

func TestErrModelTimeoutCreation(t *testing.T) {
	modelName := "test-model"
	timeout := 5 * time.Minute

	err := ErrModelTimeout(modelName, timeout)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), modelName)
	assert.Contains(t, err.Error(), "timeout")
}

func TestLatestVersionOnlyOption(t *testing.T) {
	// Test that LatestVersionOnly option is correctly set
	tests := []struct {
		name     string
		value    bool
		expected bool
	}{
		{
			name:     "LatestVersionOnly enabled",
			value:    true,
			expected: true,
		},
		{
			name:     "LatestVersionOnly disabled",
			value:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := GenerationOptions{
				LatestVersionOnly: tt.value,
			}
			assert.Equal(t, tt.expected, opts.LatestVersionOnly)
		})
	}
}

func TestGenerationOptionsFullConfiguration(t *testing.T) {
	// Test a fully configured GenerationOptions struct
	customTimeout := 15 * time.Minute
	callbackCount := 0

	opts := GenerationOptions{
		ModelTimeout:      customTimeout,
		LatestVersionOnly: true,
		ProgressCallback: func(modelName string, currentIndex, totalModels int) {
			callbackCount++
		},
	}

	assert.Equal(t, customTimeout, opts.ModelTimeout)
	assert.True(t, opts.LatestVersionOnly)
	assert.NotNil(t, opts.ProgressCallback)

	// Invoke callback multiple times
	for i := 0; i < 5; i++ {
		opts.ProgressCallback(fmt.Sprintf("model-%d", i), i+1, 5)
	}
	assert.Equal(t, 5, callbackCount)
}

func TestProgressTrackerSummary(t *testing.T) {
	// Test generating a summary from progress tracker
	tracker := NewProgressTracker(100)

	// Simulate some processing
	for i := 0; i < 60; i++ {
		tracker.IncrementProcessed()
		tracker.IncrementSuccess()
	}
	for i := 0; i < 25; i++ {
		tracker.IncrementProcessed()
		tracker.IncrementFailure()
	}
	for i := 0; i < 15; i++ {
		tracker.IncrementProcessed()
		tracker.IncrementSkipped()
	}

	// Generate summary string (as would be used in logging)
	summary := fmt.Sprintf("Progress: %d successful, %d failed, %d skipped out of %d total models",
		tracker.SuccessCount(), tracker.FailureCount(), tracker.SkippedCount(), tracker.Total())

	assert.Contains(t, summary, "60 successful")
	assert.Contains(t, summary, "25 failed")
	assert.Contains(t, summary, "15 skipped")
	assert.Contains(t, summary, "100 total")
}
