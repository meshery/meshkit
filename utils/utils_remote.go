//go:build !js

package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/meshery/meshkit/logger"
	log "github.com/sirupsen/logrus"
)

// ExtractFile extracts a tar.gz or zip archive into destDir.
func ExtractFile(filePath string, destDir string) error {
	if IsTarGz(filePath) {
		return ExtractTarGz(destDir, filePath)
	} else if IsZip(filePath) {
		return ExtractZip(destDir, filePath)
	}
	return ErrExtractType
}

func DownloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to get the file %d status code for %s file", resp.StatusCode, url)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}

// ReadFileSource supports "http", "https" and "file" protocols, returning
// the file contents as a string.
func ReadFileSource(uri string) (string, error) {
	if strings.HasPrefix(uri, "http") {
		return ReadRemoteFile(uri)
	}
	if strings.HasPrefix(uri, "file") {
		return ReadLocalFile(uri)
	}
	return "", ErrInvalidProtocol
}

// ReadRemoteFile takes in the location of a remote file in the format
// `http(s)://location/of/file` and returns the content of the file if the
// location is valid and no error occurs.
func ReadRemoteFile(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return " ", err
	}
	if response.StatusCode == http.StatusNotFound {
		return " ", ErrRemoteFileNotFound(url)
	}

	defer func() { _ = response.Body.Close() }()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return " ", ErrReadingRemoteFile(err)
	}

	return buf.String(), nil
}

// GetLatestReleaseTagsSorted returns the latest stable release tags from
// github for the given org/repo in sorted order.
func GetLatestReleaseTagsSorted(org string, repo string) ([]string, error) {
	url := "https://github.com/" + org + "/" + repo + "/releases"
	resp, err := http.Get(url)
	if err != nil {
		return nil, ErrGettingLatestReleaseTag(err)
	}
	defer safeClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, ErrGettingLatestReleaseTag(fmt.Errorf("unable to get latest release tag"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrGettingLatestReleaseTag(err)
	}
	re := regexp.MustCompile("/releases/tag/(.*?)\"")
	releases := re.FindAllString(string(body), -1)
	if len(releases) == 0 {
		return nil, ErrGettingLatestReleaseTag(errors.New("no release found in this repository"))
	}
	var versions []string
	for _, rel := range releases {
		latest := strings.ReplaceAll(rel, "/releases/tag/", "")
		latest = strings.ReplaceAll(latest, "\"", "")
		versions = append(versions, latest)
	}
	versions = SortDottedStringsByDigits(versions)
	return versions, nil
}

func safeClose(co io.Closer) {
	if cerr := co.Close(); cerr != nil {
		log.Error(cerr)
	}
}

func TrackTime(l logger.Handler, start time.Time, name string) {
	elapsed := time.Since(start)
	l.Debugf("%s took %s\n", name, elapsed)
}
