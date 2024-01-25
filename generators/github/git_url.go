package github

import (
	"bufio"
	"fmt"

	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/helm"
)

type GitURL struct {
	URL         *url.URL
	PackageName string
}

// < http/https://url/version>
//close the descriptors

func (gu GitURL) GetContent() (models.Package, error) {
	downloadDirPath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5))
	_ = os.MkdirAll(downloadDirPath, 0755)

	url := gu.URL.String()
	version := url[strings.LastIndex(url, "/")+1:]
	url, _ = strings.CutSuffix(url, "/"+version)

	fileName := utils.GetRandomAlphabetsOfDigit(6)
	downloadfilePath := filepath.Join(downloadDirPath, fileName)

	err := utils.DownloadFile(downloadfilePath, url)
	if err != nil {
		return nil, err
	}

	manifestFilePath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5)) + ".yml"

	err = ProcessDownloadedContent(downloadDirPath, downloadfilePath, manifestFilePath)
	if err != nil {
		return nil, err
	}
	return GitHubPackage{
		Name:     gu.PackageName,
		filePath: manifestFilePath,
		version:  version,
	}, nil
}

func ProcessDownloadedContent(downloadDirPath, downloadfilePath, manifestFilePath string) error {
	reader, err := os.Open(downloadfilePath)
	if err != nil {
		return utils.ErrReadFile(err, downloadfilePath)
	}
	manifestFile, _ := os.Create(manifestFilePath)
	fmt.Println("FILE PATH : ", downloadfilePath)

	w := bufio.NewWriter(manifestFile)

	defer func() {
		_ = reader.Close()
		_ = os.RemoveAll(downloadDirPath)
		_ = w.Flush()
	}()

	if utils.IsTarGz(downloadfilePath) {
		fmt.Println("yes targz : ")
		err = utils.ExtractTarGz(downloadDirPath, reader)
		fmt.Println("AFER LOADING targz", err)
	} else if utils.IsZip(downloadfilePath) {
		fmt.Println("yes zip : ")
		err = utils.ExtractZip(downloadDirPath, downloadfilePath)
		fmt.Println("AFER LOADING zip", err)
	}

	if err != nil {
		return err
	}

	utils.ProcessExtractedContent(downloadDirPath, func(path string) error {
		err := helm.ConvertToK8sManifest(path, w)
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
