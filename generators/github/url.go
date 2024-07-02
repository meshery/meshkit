package github

import (
	"bufio"
	"fmt"
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

// < http/https://url/ >
//close the descriptors

func (u URL) GetContent() (models.Package, error) {
	downloadDirPath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5))
	_ = os.MkdirAll(downloadDirPath, 0755)

	url := u.URL.String()
	owner, repo, errr := extractDetail(url)
	versions, errr := utils.GetLatestReleaseTagsSorted(owner, repo)
	if errr != nil {
		return nil, ErrInvalidGitHubSourceURL(errr)
	}
	version := versions[len(versions)-1]

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

func extractDetail(url string) (owner string, repo string, err error) {
	parts := strings.SplitN(strings.TrimPrefix(url, "/"), "/", 5)
	size := len(parts)
	if size > 4 {
		owner = parts[3]
		repo = parts[4]
	} else {
		err = ErrInvalidGitHubSourceURL(fmt.Errorf("Source URL %s is invalid", url))
	}
	return
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
