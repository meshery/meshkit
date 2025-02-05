package utils

import (
	"io"
	"os"
	"path/filepath"
)

// ProcessingContext maintains the state during content processing operations.
// It tracks which paths have been processed and provides a Writer for output.
type ProcessingContext struct {
	Writer         io.Writer
	ProcessedPaths map[string]bool
}

// NewProcessingContext creates a new context with the provided writer
func NewProcessingContext(w io.Writer) *ProcessingContext {
	return &ProcessingContext{
		Writer:         w,
		ProcessedPaths: make(map[string]bool),
	}
}

// IsProcessed checks if a given path or any of its parent directories
// have already been processed
func (pc *ProcessingContext) IsProcessed(path string) bool {
	if pc.ProcessedPaths[path] {
		return true
	}

	currentPath := path
	for {
		currentPath = filepath.Dir(currentPath)

		// Check if current directory is processed
		if pc.ProcessedPaths[currentPath] {
			return true
		}

		// Terminate at root directory
		if currentPath == "." || currentPath == "/" {
			return false
		}
	}
}

// MarkProcessed marks a path as processed in the context
func (pc *ProcessingContext) MarkProcessed(path string) {
	normalizedPath := filepath.Clean(path)
	pc.ProcessedPaths[normalizedPath] = true
}

// HandlerFunc defines the signature for content processing handlers
// It receives both the path to process and the context for state management
type HandlerFunc func(path string, ctx *ProcessingContext) error

// ProcessContent handles the processing of files and directories, managing archive extraction
// and preventing duplicate processing through the context
func ProcessPathContent(filePath string, extractPath string, ctx *ProcessingContext, handler HandlerFunc) error {
	// Skip if already processed
	if ctx.IsProcessed(filePath) {
		return nil
	}

	var err error
	var pathToProcess string

	if IsTarGz(filePath) {
		err = ExtractTarGz(extractPath, filePath)
		pathToProcess = extractPath
	} else if IsZip(filePath) {
		err = ExtractZip(extractPath, filePath)
		pathToProcess = extractPath
	} else {
		// For regular files, use the file path directly
		pathToProcess = filePath
	}

	if err != nil {
		return err
	}

	pathInfo, err := os.Stat(filePath)
	if err != nil {
		return ErrReadDir(err, filePath)
	}

	// Process directory or file
	if pathInfo.IsDir() {
		entries, err := os.ReadDir(filePath)
		if err != nil {
			return ErrReadDir(err, filePath)
		}

		for _, entry := range entries {
			err := ProcessPathContent(
				filepath.Join(pathToProcess, entry.Name()),
				filepath.Join(extractPath, entry.Name()),
				ctx,
				handler,
			)
			if err != nil {
				return err
			}
		}
	} else {
		// Process individual file
		err := handler(filePath, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
