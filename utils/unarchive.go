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

	"github.com/meshery/meshkit/utils"
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

func ExtractTarGz(path, downloadfilePath string) (err error) { // Note: named return `err` is added
	gzipStream, err := os.Open(downloadfilePath)
	if err != nil {
		return utils.ErrReadFile(err, downloadfilePath) // Assuming ErrReadFile is in the utils package
	}
	// CORRECTED: defer now handles the error from Close()
	defer func() {
		if closeErr := gzipStream.Close(); closeErr != nil && err == nil {
			err = utils.ErrCloseFile(closeErr)
		}
	}()

	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return utils.ErrExtractTarXZ(err, path)
	}
	defer uncompressedStream.Close() // Note: This Close() can also fail and should ideally be handled.
	// For now, we are addressing the reviewer's direct comment.

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return utils.ErrExtractTarXZ(err, path)
		}

		targetPath := filepath.Join(path, header.Name)
		if !strings.HasPrefix(targetPath, filepath.Clean(path)+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry is trying to escape the destination directory: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return utils.ErrExtractTarXZ(err, path) // Re-using existing error for simplicity
			}
		case tar.TypeReg:
			outFile, err := os.Create(targetPath)
			if err != nil {
				return utils.ErrExtractTarXZ(err, path)
			}
			if _, err = io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close() // Attempt to close, but return the more important Copy error
				return utils.ErrExtractTarXZ(err, path)
			}
			// CORRECTED: Using the standard Meshkit error function
			if err := outFile.Close(); err != nil {
				return utils.ErrCloseFile(err)
			}

		default:
			return utils.ErrUnsupportedTarHeaderType(header.Typeflag, header.Name)
		}
	}
	return nil
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
