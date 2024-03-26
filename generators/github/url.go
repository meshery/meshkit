package github

import (
	"bufio"
	"io"

	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/generators/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/helm"
)

type URL struct {
	URL         *url.URL
	PackageName string
}

// < http/https://url/version>
//close the descriptors

func (u URL) GetContent() (models.Package, error) {
	downloadDirPath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5))
	_ = os.MkdirAll(downloadDirPath, 0755)

	url := u.URL.String()
	version := url[strings.LastIndex(url, "/")+1:]
	url, _ = strings.CutSuffix(url, "/"+version)

	fileName := utils.GetRandomAlphabetsOfDigit(6)
	downloadfilePath := filepath.Join(downloadDirPath, fileName)

	err := utils.DownloadFile(downloadfilePath, url)
	if err != nil {
		return nil, err
	}

	manifestFilePath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5)) + ".yml"

	manifestFile, _ := os.Create(manifestFilePath)

	w := bufio.NewWriter(manifestFile)

	defer func() {
		_ = os.RemoveAll(downloadDirPath)
		_ = w.Flush()
	}()

	err = ProcessContent(w, downloadDirPath, downloadfilePath)
	if err != nil {
		return nil, err
	}
	return GitHubPackage{
		Name:      u.PackageName,
		filePath:  manifestFilePath,
		version:   version,
		SourceURL: u.URL.String(),
	}, nil
}

func ProcessContent(w io.Writer, downloadDirPath, downloadfilePath string) error {
	var err error
	if utils.IsTarGz(downloadfilePath) {
		err = utils.ExtractTarGz(downloadDirPath, downloadfilePath)
	} else if utils.IsZip(downloadfilePath) {
		err = utils.ExtractZip(downloadDirPath, downloadfilePath)
	} else {
		// If it is not an archive/zip, then the file itself is to be processed.
		downloadDirPath = downloadfilePath
	}

	if err != nil {
		return err
	}

	err = utils.ProcessContent(downloadDirPath, func(path string) error {
		err = helm.ConvertToK8sManifest(path, w)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}
	return nil
}
