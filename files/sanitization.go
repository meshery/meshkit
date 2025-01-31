package files

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	// "path"

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
	FileExt string
	RawData []byte
	// incase of bundle like tar
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
		err := IsValidYaml(data)
		if err != nil {
			return SanitizedFile{}, fmt.Errorf("File is not valid yaml: %w", err)
		}

		return SanitizedFile{
			FileExt: ext,
			RawData: data,
		}, nil

	case ".json":

		err := IsValidJson(data)
		if err != nil {
			return SanitizedFile{}, fmt.Errorf("File is not valid json: %w", err)
		}

		return SanitizedFile{
			FileExt: ext,
			RawData: data,
		}, nil

	case ".tar", ".tar.gz", ".zip", ".gz", ".tgz":

		return SanitizeBundle(data, fileName, ext, tempDir)

	}

	return SanitizedFile{}, fmt.Errorf("Unsupported file extension %s", ext)

}

// ExtractTar extracts a .tar, .tar.gz, or .tgz file into a temporary directory and returns the directory.
func ExtractTar(reader io.Reader, archiveFile string, parentTempDir string) (*os.File, error) {
	// Open the archive file

	// Determine if the file is compressed (gzip)
	if strings.HasSuffix(archiveFile, ".gz") || strings.HasSuffix(archiveFile, ".tgz") {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Create a temporary directory to extract the files
	tempDir, err := os.MkdirTemp(parentTempDir, archiveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %v", err)
	}

	// Create a tar reader
	tarReader := tar.NewReader(reader)

	// Track the top-level directory in the archive
	var topLevelDir string
	// Iterate through the tar archive
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar archive: %v", err)
		}

		// Construct the full path for the file/directory
		targetPath := filepath.Join(tempDir, header.Name)

		// If this is the first entry, determine the top-level directory
		if topLevelDir == "" {
			topLevelDir = filepath.Dir(header.Name)
		}

		// Ensure the parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create parent directory: %v", err)
		}

		// Handle directories
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
				return nil, fmt.Errorf("failed to create directory: %v", err)
			}
			continue
		}

		// Handle regular files
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %v", err)
		}
		defer file.Close()

		// Copy file contents
		if _, err := io.Copy(file, tarReader); err != nil {
			return nil, fmt.Errorf("failed to copy file contents: %v", err)
		}
	}

	// If the archive contains a top-level directory, return its path
	if topLevelDir != "" {
		topLevelPath := filepath.Join(tempDir, topLevelDir)
		extractedDir, err := os.Open(topLevelPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open extracted directory: %v", err)
		}
		return extractedDir, nil
	}

	// If no top-level directory is found, return the root tempDir
	extractedDir, err := os.Open(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open extracted directory: %v", err)
	}

	return extractedDir, nil
}

func SanitizeBundle(data []byte, fileName string, ext string, tempDir string) (SanitizedFile, error) {
	// fmt.Println("temp", tempDir)
	extracted, err := ExtractTar(bytes.NewReader(data), fileName, tempDir)

	return SanitizedFile{
		FileExt:          ext,
		ExtractedContent: extracted,
		RawData:          data,
	}, err
}

func IsValidYaml(data []byte) error {
	var unMarshalled interface{}
	return yaml.Unmarshal(data, &unMarshalled)
}

func IsValidJson(data []byte) error {
	var unMarshalled interface{}
	return json.Unmarshal(data, &unMarshalled)
}
