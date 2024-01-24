package github

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
)

type GitURL struct {
	URL         *url.URL
	PackageName string
}

// < http/https://url/version>
func (gu GitURL) GetContent() (models.Package, error) {
	path := os.TempDir()
	url := gu.URL.String()
	version := url[strings.LastIndex(url, "/")+1:]
	url, _ = strings.CutSuffix(url, "/"+version)
	fileName := utils.GetRandomAlphabetsOfDigit(6)
	filePath := filepath.Join(path, fileName)
	err := utils.DownloadFile(filePath, url)
	if err != nil {
		return nil, err
	}
	return GitHubPackage{
		Name:       gu.PackageName,
		filePath:   filePath,
		version:    version,
	}, nil
}
