package oci

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	archiveTar "archive/tar"

	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/crane"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/meshery/meshkit/utils"
)

func CreateTempOCIContentDir() (tempDir string, err error) {
	wd := utils.GetHome()	
	wd = filepath.Join(wd, ".meshery", "content")
	
	if err := os.MkdirAll(wd, 0755); err != nil {
		return "", err
	}
	
	tempDir, err = os.MkdirTemp(wd, "oci")
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

// saves the oci artifact to the given path as tarball type
func SaveOCIArtifact(img gcrv1.Image, artifactPath, name string) error {
	repoWithTag := fmt.Sprintf("%s:%s", name, "latest") // TODO: Add support to make this dynamic from user input
	err := crane.Save(img, repoWithTag, artifactPath)
	if err != nil {
		return ErrSavingImage(err)
	}

	return nil
}

// uncompress the OCI Artifact tarball at the given destination path
func UnCompressOCIArtifact(source, destination string) error {
	img, err := tarball.ImageFromPath(source, nil)
	if err != nil {
		return ErrGettingImage(err)
	}

	opt := []validate.Option{}

	if err = validate.Image(img, opt...); err != nil {
		return ErrValidatingImage(err)
	}

	layers, err := img.Layers()
	if err != nil {
		return ErrGettingLayer(err)
	}

	blob, err := layers[0].Compressed()
	if err != nil {
		return ErrCompressingLayer(err)
	}

	if err = tar.Untar(blob, destination, tar.WithMaxUntarSize(-1), tar.WithSkipSymlinks()); err != nil {
		return ErrUnTaringLayer(err)
	}

	return nil
}

// ValidateOCIArtifact validates the OCI artifact tarball using go-containerregistry's validate function
func ValidateOCIArtifact(tarballPath string) error {
	img, err := tarball.ImageFromPath(tarballPath, nil)
	if err != nil {
		return err
	}

	return validate.Image(img)
}

// IsOCIArtifact checks if the tarball is an OCI artifact by looking for manifest.json or index.json
func IsOCIArtifact(data []byte) bool {
	reader := bytes.NewReader(data)
	var tr *archiveTar.Reader

	if gzr, err := gzip.NewReader(reader); err == nil {
		defer gzr.Close()
		tr = archiveTar.NewReader(gzr)
	} else {
		_, err := reader.Seek(0, io.SeekStart)
		if err != nil {
			return false
		}
		tr = archiveTar.NewReader(reader)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}

		if header.Name == "manifest.json" || header.Name == "index.json" {
			return true
		}
	}

	return false
}

func ExtractAndValidateManifest(data []byte) error {
	reader := bytes.NewReader(data)
	var tr *archiveTar.Reader

	if gzr, err := gzip.NewReader(reader); err == nil {
		defer gzr.Close()
		tr = archiveTar.NewReader(gzr)
	} else {
		_, err := reader.Seek(0, io.SeekStart)
		if err != nil {
			return ErrSeekFailed(err)
		}
		tr = archiveTar.NewReader(reader)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Name == "manifest.json" {
			var manifest []map[string]interface{}
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return fmt.Errorf("failed to decode manifest.json: %w", err)
			}
			fmt.Println("manifest.json is valid:", manifest)
			return nil
		}
	}

	return fmt.Errorf("manifest.json not found in tarball")
}
