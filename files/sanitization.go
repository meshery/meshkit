package files

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"sort"

	"fmt"
	"io"

	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SanitizedFile struct {
	FileExt  string
	FileName string
	RawData  []byte
	// incase of bundle like tar
	ExtractedContentPath string
}

var ValidIacExtensions = map[string]bool{
	".yml":     true,
	".yaml":    true,
	".json":    true,
	".tar":     true,
	".tar.gz":  true,
	".tar.tgz": true,
	".zip":     true,
	".gz":      true,
	".tgz":     true,
}

func identifyExtension(name string) string {

	normalizedName := strings.ToLower(strings.TrimSpace(name))

	// Prioritize longer, more specific extensions first to correctly identify ".tar.gz" over ".gz"
	knownExtensions := []string{".tar.gz", ".tar.tgz", ".tgz", ".tar", ".zip", ".gz", ".yml", ".yaml", ".json"}
	for _, ext := range knownExtensions {
		if strings.HasSuffix(normalizedName, ext) {
			return ext
		}
	}

	return filepath.Ext(normalizedName)
}

func SanitizeFile(data []byte, fileName string, tempDir string, validExts map[string]bool) (SanitizedFile, error) {

	ext := identifyExtension(fileName)

	// 1. Check if file has supported  extension
	if !validExts[ext] {
		return SanitizedFile{}, ErrUnsupportedExtension(fileName, ext, validExts)
	}
	switch ext {

	case ".yml", ".yaml":
		err := IsValidYaml(data)
		if err != nil {
			return SanitizedFile{}, ErrInvalidYaml(fileName, err)
		}

		return SanitizedFile{
			FileExt:  ext,
			FileName: fileName,
			RawData:  data,
		}, nil

	case ".json":

		err := IsValidJson(data)
		if err != nil {
			return SanitizedFile{}, ErrInvalidJson(fileName, err)
		}

		return SanitizedFile{
			FileExt:  ext,
			RawData:  data,
			FileName: fileName,
		}, nil

	case ".tar", ".tar.gz", ".zip", ".gz", ".tgz":

		return SanitizeBundle(data, fileName, ext, tempDir)

	}

	return SanitizedFile{}, ErrUnsupportedExtension(fileName, ext, validExts)

}

// ExtractTar extracts a .tar, .tar.gz, or .tgz file into a temporary directory and returns the directory.
func ExtractTar(reader io.Reader, archiveFile string, outputDir string) error {
	// Open the archive file

	// Determine if the file is compressed (gzip)
	if strings.HasSuffix(archiveFile, ".gz") || strings.HasSuffix(archiveFile, ".tgz") {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Create a tar reader
	tarReader := tar.NewReader(reader)

	// Iterate through the tar archive
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		// Construct the full path for the file/directory
		targetPath := filepath.Join(outputDir, header.Name)

		// Ensure the parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create parent directory: %v", err)
		}

		// Handle directories
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}
			continue
		}

		// Handle regular files
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to create file: %v", err)
		}
		defer file.Close()

		// Copy file contents
		if _, err := io.Copy(file, tarReader); err != nil {
			return fmt.Errorf("failed to copy file contents: %v", err)
		}
	}

	return nil
}

// ExtractZipFromBytes takes a []byte representing a ZIP file and extracts it to the specified output directory.
func ExtractZipFromBytes(data []byte, outputDir string) error {
	// Create a bytes.Reader from the input byte slice
	reader := bytes.NewReader(data)

	// Open the ZIP archive from the reader
	zipReader, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %w", err)
	}

	// Iterate through the files in the ZIP archive
	for _, file := range zipReader.File {
		// Construct the output file path
		filePath := filepath.Join(outputDir, file.Name)

		// Check if the file is a directory
		if file.FileInfo().IsDir() {
			// Create the directory
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create the parent directories if they don't exist
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}

		// Open the file in the ZIP archive
		zipFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %w", err)
		}
		defer zipFile.Close()

		// Create the output file
		outFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		// Copy the contents of the file from the ZIP archive to the output file
		if _, err := io.Copy(outFile, zipFile); err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}
	}

	return nil
}

// SanitizeDirNames replaces spaces in directory names with hyphens recursively in the specified root directory.
func SanitizeDirNames(root string) error {
	var dirPaths []string

	// Walk the directory tree and collect all directory paths
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirPaths = append(dirPaths, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan directory tree: %w", err)
	}

	// Sort directories by depth (number of path separators) in descending order
	// This ensures that we rename deeper directories first, avoiding issues with renaming parent directories before
	sort.SliceStable(dirPaths, func(i, j int) bool {
		return strings.Count(dirPaths[i], string(os.PathSeparator)) >
			strings.Count(dirPaths[j], string(os.PathSeparator))
	})

	// Iterate through the collected directory paths and rename them if they contain spaces
	for _, path := range dirPaths {
		base := filepath.Base(path)
		if strings.Contains(base, " ") {
			safeBase := strings.ReplaceAll(base, " ", "-")
			newPath := filepath.Join(filepath.Dir(path), safeBase)
			if path != newPath {
				if err := os.Rename(path, newPath); err != nil {
					return fmt.Errorf("failed to rename %q to %q: %w", path, newPath, err)
				}
			}
		}
	}

	return nil
}

// removeHiddenFiles deletes .DS_Store and AppleDouble (._*) files recursively
func removeHiddenFiles(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "._") || base == ".DS_Store" || base == "Icon\r" {
			if remErr := os.Remove(path); remErr != nil {
				return fmt.Errorf("failed to remove hidden macOS file %q: %w", path, remErr)
			}
		}

		return nil
	})
}

// get the root dir from the extractedPath
// if multiple entries are found in the extractedPath then treat extractedPath as root
func GetFirstTopLevelDir(extractedPath string) (string, error) {
	entries, err := os.ReadDir(extractedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted directory: %v", err)
	}

	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(extractedPath, entries[0].Name()), nil
	}
	return extractedPath, nil
}

func SanitizeBundle(data []byte, fileName string, ext string, tempDir string) (SanitizedFile, error) {

	outputDir, err := os.MkdirTemp(tempDir, fileName)

	if err != nil {
		return SanitizedFile{}, ErrFailedToExtractArchive(fileName, fmt.Errorf("failed to create extraction directory %w", err))
	}

	switch ext {

	case ".tar", ".tar.gz", ".tgz", ".gz":
		err = ExtractTar(bytes.NewReader(data), fileName, outputDir)
	case ".zip":
		err = ExtractZipFromBytes(data, outputDir)
	default:
		return SanitizedFile{}, ErrFailedToExtractArchive(fileName, fmt.Errorf("unsupported compression extension %s", ext))
	}

	if err != nil {
		return SanitizedFile{}, err
	}
	// Sanitize directory names in the extracted content
	if err := SanitizeDirNames(outputDir); err != nil {
		return SanitizedFile{}, fmt.Errorf("failed to sanitize directories: %w", err)
	}

	// Remove hidden macOS files like .DS_Store and AppleDouble (._*)
	if err := removeHiddenFiles(outputDir); err != nil {
		return SanitizedFile{}, fmt.Errorf("failed to remove hidden macOS files from extractedPath: %w", err)
	}

	// jump directly into the extracted dirs toplevel dir (which is the case if a single folder is archived the extracted dir preserves that structure)

	extractedPath, err := GetFirstTopLevelDir(outputDir)

	if err != nil {
		return SanitizedFile{}, ErrFailedToExtractArchive(fileName, err)
	}
	
	return SanitizedFile{
		FileExt:              ext,
		FileName:             fileName,
		ExtractedContentPath: extractedPath,
		RawData:              data,
	}, nil

}

func IsValidYaml(data []byte) error {
	var unMarshalled interface{}
	return yaml.Unmarshal(data, &unMarshalled)
}

func IsValidJson(data []byte) error {
	var unMarshalled interface{}
	return json.Unmarshal(data, &unMarshalled)
}
