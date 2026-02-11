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
	defer zipReader.Close()
	destDir, err := filepath.Abs(path)
	if err != nil {
		return ErrExtractZip(err, path)
	}
	for _, file := range zipReader.File {
		err := func() error {
			targetPath, err := filepath.Abs(filepath.Join(destDir, file.Name))
			if err != nil {
				return err
			}

			if !strings.HasPrefix(targetPath, destDir+string(os.PathSeparator)) && targetPath != destDir {
				return fmt.Errorf("zipslip: illegal file path: %s", file.Name)
			}

			// CHECK for files to skip (macOS metadata)
			if strings.HasPrefix(filepath.Base(targetPath), "._") || filepath.Base(targetPath) == "__MACOSX" {
				return nil
			}

			fd, err := file.Open()
			if err != nil {
				return err
			}
			defer fd.Close()

			if file.FileInfo().IsDir() {
				return os.MkdirAll(targetPath, file.Mode())
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			openedFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer openedFile.Close()

			_, err = io.Copy(openedFile, fd)
			return err
		}()
		if err != nil {
			return ErrExtractZip(err, path)
		}
	}
	return nil
}

func ExtractTarGz(path, downloadfilePath string) error {
	gzipStream, err := os.Open(downloadfilePath)
	if err != nil {
		return ErrReadFile(err, downloadfilePath)
	}
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return ErrExtractTarXZ(err, path)
	}
	defer func() {
		_ = uncompressedStream.Close()
		_ = gzipStream.Close()
	}()

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
			_ = os.MkdirAll(filepath.Join(path, filepath.Dir(header.Name)), 0755)
			var outFile *os.File
			outFile, err = os.Create(filepath.Join(path, header.Name))
			if err != nil {
				return ErrExtractTarXZ(err, path)
			}
			if _, err = io.Copy(outFile, tarReader); err != nil {
				return ErrExtractTarXZ(err, path)
			}
			_ = outFile.Close()

		default:
			return ErrExtractTarXZ(err, path)
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
