package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func IsTarGz(name string) bool {
	buffer, err := readData(name)
	if err != nil && err != io.EOF {
		return false
	}

	if err != nil && err != io.EOF {
		return false
	}

	if contentType := http.DetectContentType(buffer); contentType != "application/x-gzip" {
		return false
	}
	return true
}

func IsZip(name string) bool {
	buffer, err := readData(name)
	if err != nil && err != io.EOF {
		return false
	}

	if contentType := http.DetectContentType(buffer); contentType != "application/zip" {
		return false
	}
	return true
}

func IsYaml(name string) bool {
	buffer, err := readData(name)
	if err != nil && err != io.EOF {
		return false
	}

	if contentType := http.DetectContentType(buffer); !strings.Contains(contentType, "text/plain") {
		return false
	}
	return true
}

func readData(name string) (buffer []byte, err error) {
	raw, err := os.Open(name)
	if err != nil {
		return
	}

	defer func() {
		_ = raw.Close()
		_, _ = raw.Seek(0, 0)
	}()
	buffer = make([]byte, 512)
	_, err = raw.Read(buffer)

	if err != nil {
		return
	}
	return
}

func ExtractZip(path, artifactPath string) error {
	zipReader, err := zip.OpenReader(artifactPath)
	if err != nil {
		return ErrExtractZip(err, path)
	}
	defer func() {
		_ = zipReader.Close()
	}()

	buffer := make([]byte, 1<<4)
	for _, file := range zipReader.File {

		fd, err := file.Open()
		if err != nil {
			return ErrExtractZip(err, path)
		}
		defer func() {
			_ = fd.Close()
		}()

		filePath := filepath.Join(path, file.Name)

		if file.FileInfo().IsDir() {
			err := os.Mkdir(file.Name, file.Mode())
			if err != nil {
				return ErrExtractZip(err, path)
			}
		} else {
			openedFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return ErrExtractZip(err, path)
			}
			_, err = io.CopyBuffer(openedFile, fd, buffer)
			if err != nil {
				// We need to close the file, but the error from CopyBuffer is more important.
				_ = openedFile.Close()
				return ErrExtractZip(err, path)
			}
			if err := openedFile.Close(); err != nil {
				return ErrExtractZip(err, path)
			}
		}
	}
	return nil
}

func ExtractTarGz(path, downloadfilePath string) error {
	gzipStream, err := os.Open(downloadfilePath)
	if err != nil {
		return ErrReadFile(err, downloadfilePath)
	}
	defer gzipStream.Close()

	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return ErrExtractTarXZ(err, path)
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return ErrExtractTarXZ(err, path)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(filepath.Join(path, header.Name), 0755); err != nil {
				return ErrExtractTarXZ(err, path)
			}
		case tar.TypeReg:
			// Ensure that the directory for the file exists
			_ = os.MkdirAll(filepath.Join(path, filepath.Dir(header.Name)), 0755)
			var outFile *os.File
			outFile, err = os.Create(filepath.Join(path, header.Name))
			if err != nil {
				return ErrExtractTarXZ(err, path)
			}
			if _, err = io.Copy(outFile, tarReader); err != nil {
				// The subsequent Close() call will handle closing the file.
				// Return the more critical I/O error immediately.
				_ = outFile.Close() // Attempt to close, but ignore error as we are returning the copy error.
				return ErrExtractTarXZ(err, path)
			}

			// Close the file and check for errors
			if err := outFile.Close(); err != nil {
				return ErrFileClose(err, header.Name)
			}

		default:
			return ErrUnsupportedTarHeaderType(header.Typeflag, header.Name)
		}
	}

	return nil
}

// ErrFileClose creates a standardized error for file closing failures.
func ErrFileClose(err error, filename string) error {
	return fmt.Errorf("failed to close file %s: %w", filename, err)
}

// ErrUnsupportedTarHeaderType creates an error for unsupported tar header types.
func ErrUnsupportedTarHeaderType(typeflag byte, name string) error {
	return fmt.Errorf("unsupported tar header type %v for file %s", typeflag, name)
}

func ProcessContent(filePath string, f func(path string) error) error {
	pathInfo, err := os.Stat(filePath)
	if err != nil {
		return ErrReadDir(err, filePath)
	}
	if pathInfo.IsDir() {
		entries, err := os.ReadDir(filePath)
		if err != nil {
			return ErrReadDir(err, filePath)
		}

		for _, entry := range entries {
			err := f(filepath.Join(filePath, entry.Name()))
			if err != nil {
				return err
			}
		}
	} else {
		err := f(filePath)
		if err != nil {
			return err
		}
	}
	return nil
}
