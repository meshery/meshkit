package files

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	// "errors"
	"fmt"
	"io"
	// "io/ioutil"
	// "mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SanitizedFile struct {
	FileExt          string
	UnmarshalledFile interface{}
	ExtractedContent *os.File
}

func SanitizeFile(data []byte, fileName string, tempDir string) (SanitizedFile, error) {

	validExts := map[string]bool{
		".json":   true,
		".yml":    true,
		".yaml":   true,
		".tar":    true,
		".tar.gz": true,
		".tgz":    true,
		".zip":    true,
	}

	ext := filepath.Ext(fileName)

	// 1. Check if file has supported  extension
	if !validExts[ext] && !validExts[filepath.Ext(strings.TrimSuffix(fileName, ".gz"))] {
		return SanitizedFile{}, fmt.Errorf("unsupported file extension: %s", ext)
	}

	switch ext {

	case ".yml", ".yaml":
		var unMarshalled interface{}
		err := yaml.Unmarshal(data, &unMarshalled)

		if err != nil {
			return SanitizedFile{}, err
		}

		return SanitizedFile{
			UnmarshalledFile: unMarshalled,
			FileExt:          ext,
		}, nil

	case ".json":
		var unMarshalled interface{}
		err := json.Unmarshal(data, &unMarshalled)

		if err != nil {
			return SanitizedFile{}, err
		}

		return SanitizedFile{
			UnmarshalledFile: unMarshalled,
			FileExt:          ext,
		}, nil

	case ".tar", ".tar.gz", ".zip", ".gz":

		return SanitizeBundle(data, fileName, ext, tempDir)

	}

	return SanitizedFile{}, fmt.Errorf("Unsupported file extension %s", ext)

}

// will skip nested dirs right now
func SanitizeBundle(data []byte, fileName string, ext string, tempDir string) (SanitizedFile, error) {

	var tarReader *tar.Reader
	input := bytes.NewReader(data)

	if strings.HasSuffix(fileName, ".gz") || strings.HasSuffix(fileName, ".tgz") {
		gzReader, err := gzip.NewReader(input)
		if err != nil {
			return SanitizedFile{}, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(input)
	}

	extractDir, _ := os.MkdirTemp(tempDir, fileName)

	// Extract and validate files in the bundle
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return SanitizedFile{}, fmt.Errorf("failed to read tar entry: %w", err)
		}

		switch header.Typeflag {

		case tar.TypeDir: // If it's a directory, create it

			target := filepath.Join(extractDir, header.Name)
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return SanitizedFile{}, err
			}

		case tar.TypeReg:
			ext := filepath.Ext(header.Name)
			if ext != ".json" && ext != ".yaml" && ext != ".yml" {
				continue
			}

			// read the complete content of the file h.Name into the bs []byte
			fileBytes, _ := io.ReadAll(tarReader) // Validate the extracted file

			if _, err := SanitizeFile(fileBytes, header.Name, tempDir); err != nil {
				fmt.Printf("Skipping invalid file: %s\n", header.Name)
				continue
			}

			// Create a temporary file for the extracted content
			tempFilePath := filepath.Join(extractDir, header.Name)
			tempFile, err := os.Create(tempFilePath)

			// fail forward and continue
			if err != nil {
				fmt.Println("failed to create temp file: %w", err)
				continue
			}
			defer tempFile.Close()

			// Write the extracted content to the temp file
			if _, err := io.Copy(tempFile, tarReader); err != nil {
				fmt.Println("failed to write to temp file: %w", err)
			}
		}

	}

	// Return a handle to the temp directory (or a specific file if needed)
	extractedContent, err := os.Open(tempDir)

	if err == nil {
		return SanitizedFile{
			FileExt:          ext,
			UnmarshalledFile: nil,
			ExtractedContent: extractedContent,
		}, nil
	}

	return SanitizedFile{}, fmt.Errorf("Failed to open extracted dir %w", err)
}
