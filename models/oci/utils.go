package oci

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/crane"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/layer5io/meshkit/utils"
)

func CreateTempOCIContentDir() (tempDir string, err error) {
	wd := utils.GetHome()
	wd = filepath.Join(wd, ".meshery", "content")
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
		return err
	}

	return nil
}

// uncompress the OCI Artifact tarball at the given path
func UnCompressOCIArtifact(img gcrv1.Image, destination string) error {
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
