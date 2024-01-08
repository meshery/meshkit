package oci

import (
	"os"
	"path/filepath"
	"fmt"

	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/crane"
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