package github

import (
	"bufio"

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

	// Parse URL and extract version information
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

	// Create processing context and handler
	ctx := utils.NewProcessingContext(w)
	handler := func(path string, ctx *utils.ProcessingContext) error {
		return helm.ConvertToK8sManifest(path, "", ctx)
	}

	defer func() {
		_ = os.RemoveAll(downloadDirPath)
		_ = w.Flush()
	}()

	err = utils.ProcessPathContent(downloadfilePath, downloadDirPath, ctx, handler)
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
