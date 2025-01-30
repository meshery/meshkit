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

	// Iterate through the tar archive
	for {
		header, err := tarReader.Next()
		// fmt.Println("Header name %s", header.Name)
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar archive: %v", err)
		}

		// Construct the full path for the file/directory
		targetPath := filepath.Join(tempDir, header.Name)

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

	// Return the temporary directory as an *os.File
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

// // will skip nested dirs right now
// func SanitizeBundle_(data []byte, fileName string, ext string, tempDir string) (SanitizedFile, error) {

// 	var tarReader *tar.Reader
// 	input := bytes.NewReader(data)

// 	if strings.HasSuffix(fileName, ".gz") || strings.HasSuffix(fileName, ".tgz") {
// 		gzReader, err := gzip.NewReader(input)
// 		if err != nil {
// 			return SanitizedFile{}, fmt.Errorf("failed to create gzip reader: %w", err)
// 		}
// 		defer gzReader.Close()
// 		tarReader = tar.NewReader(gzReader)
// 	} else {
// 		tarReader = tar.NewReader(input)
// 	}

// 	// cleaning up is resposibility of the tempDir owner
// 	extractDir, _ := os.MkdirTemp(tempDir, fileName)

// 	// Extract and validate files in the bundle
// 	for {
// 		header, err := tarReader.Next()
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return SanitizedFile{}, fmt.Errorf("failed to read tar entry: %w", err)
// 		}

// 		target := filepath.Join(extractDir, header.Name)
// 		// Handle directories
// 		if header.FileInfo().IsDir() {
// 			if err := os.MkdirAll(target, os.ModePerm); err != nil {
// 				return SanitizedFile{}, fmt.Errorf("failed to create directory: %v", err)
// 			}
// 			continue
// 		}

// 		switch header.Typeflag {

// 		case tar.TypeDir: // If it's a directory, create it

// 			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
// 				return SanitizedFile{}, err
// 			}

// 		case tar.TypeReg:
// 			ext := filepath.Ext(header.Name)
// 			if ext != ".json" && ext != ".yaml" && ext != ".yml" {
// 				continue
// 			}

// 			// read the complete content of the file h.Name into the bs []byte
// 			// fileBytes, _ := io.ReadAll(tarReader) // Validate the extracted file

// 			// if _, err := SanitizeFile(fileBytes, header.Name, tempDir); err != nil {
// 			// 	fmt.Printf("Skipping invalid file: %s\n", header.Name)
// 			// 	continue
// 			// }

// 			// Create a temporary file for the extracted content
// 			tempFile, err := os.Create(target)

// 			// fail forward and continue
// 			if err != nil {
// 				fmt.Println("failed to create temp file: %w", err)
// 				continue
// 			}
// 			defer tempFile.Close()

// 			// Write the extracted content to the temp file
// 			if _, err := io.Copy(tempFile, tarReader); err != nil {
// 				fmt.Println("failed to write to temp file: %w", err)
// 			}
// 		}

// 	}

// 	// Return a handle to the temp directory (or a specific file if needed)
// 	extractedContent, err := os.Open(tempDir)

// 	if err == nil {
// 		return SanitizedFile{
// 			FileExt:          ext,
// 			RawData:          data,
// 			ExtractedContent: extractedContent,
// 		}, nil
// 	}

// 	return SanitizedFile{}, fmt.Errorf("Failed to open extracted dir %w", err)
// }

func IsValidYaml(data []byte) error {
	var unMarshalled interface{}
	return yaml.Unmarshal(data, &unMarshalled)
}

func IsValidJson(data []byte) error {
	var unMarshalled interface{}
	return json.Unmarshal(data, &unMarshalled)
}
