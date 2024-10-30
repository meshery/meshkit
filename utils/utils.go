package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/layer5io/meshkit/models/meshmodel/entity"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// transforms the keys of a Map recursively with the given transform function
func TransformMapKeys(input map[string]interface{}, transformFunc func(string) string) map[string]interface{} {
	output := make(map[string]interface{})
	for k, v := range input {
		transformedKey := transformFunc(k)
		value, ok := v.(map[string]interface{})
		if !ok {
			output[transformedKey] = v
		} else {
			output[transformedKey] = TransformMapKeys(value, transformFunc)
		}
	}
	return output
}

// Deprecated: Use Unmarshal from encoding package.
// TODO: Replace the usages from all projects.
func Unmarshal(obj string, result interface{}) error {
	obj = strings.TrimSpace(obj)
	err := json.Unmarshal([]byte(obj), result)
	if err != nil {
		if e, ok := err.(*json.SyntaxError); ok {
			return ErrUnmarshalSyntax(err, e.Offset)
		}
		if e, ok := err.(*json.UnmarshalTypeError); ok {
			return ErrUnmarshalType(err, e.Value)
		}
		if e, ok := err.(*json.UnsupportedTypeError); ok {
			return ErrUnmarshalUnsupportedType(err, e.Type)
		}
		if e, ok := err.(*json.UnsupportedValueError); ok {
			return ErrUnmarshalUnsupportedValue(err, e.Value)
		}
		if e, ok := err.(*json.InvalidUnmarshalError); ok {
			return ErrUnmarshalInvalid(err, e.Type)
		}
		return ErrUnmarshal(err)
	}
	return nil
}

// getBool function returns the boolean config data
func GetBool(key string) (bool, error) {
	enabled, err := strconv.ParseBool(key)
	if err != nil {
		return false, ErrGetBool(key, err)
	}

	return enabled, nil
}

func StrConcat(s ...string) string {
	var buf strings.Builder

	for _, str := range s {
		buf.WriteString(str)
	}
	return buf.String()
}

func Marshal(obj interface{}) (string, error) {
	result, err := json.Marshal(obj)
	if err != nil {
		return " ", ErrMarshal(err)
	}
	return string(result), nil
}

func Filepath() string {
	_, fn, line, _ := runtime.Caller(0)

	return fmt.Sprintf("file: %s, line: %d", fn, line)
}

func DownloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to get the file %d status code for %s file", resp.StatusCode, url)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// GetHome returns the home path
func GetHome() string {
	usr, _ := user.Current()
	return usr.HomeDir
}

// CreateFile creates a file with the given content on the given location with
// the given filename
func CreateFile(contents []byte, filename string, location string) error {
	// Create file in -rw-r--r-- mode
	fd, err := os.OpenFile(filepath.Join(location, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err = fd.Write(contents); err != nil {
		fd.Close()
		return err
	}

	if err = fd.Close(); err != nil {
		return err
	}

	return nil
}

// ReadFileSource supports "http", "https" and "file" protocols.
// it takes in the location as a uri and returns the contents of
// file as a string.
func ReadFileSource(uri string) (string, error) {
	if strings.HasPrefix(uri, "http") {
		return ReadRemoteFile(uri)
	}
	if strings.HasPrefix(uri, "file") {
		return ReadLocalFile(uri)
	}

	return "", ErrInvalidProtocol
}

// ReadRemoteFile takes in the location of a remote file
// in the format 'http://location/of/file' or 'https://location/file'
// and returns the content of the file if the location is valid and
// no error occurs
func ReadRemoteFile(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return " ", err
	}
	if response.StatusCode == http.StatusNotFound {
		return " ", ErrRemoteFileNotFound(url)
	}

	defer response.Body.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return " ", ErrReadingRemoteFile(err)
	}

	return buf.String(), nil
}

// ReadLocalFile takes in the location of a local file
// in the format `file://location/of/file` and returns
// the content of the file if the path is valid and no
// error occurs
func ReadLocalFile(location string) (string, error) {
	// remove the protocol prefix
	location = strings.TrimPrefix(location, "file://")

	// Need to support variable file locations hence
	// #nosec
	data, err := os.ReadFile(location)
	if err != nil {
		return "", ErrReadingLocalFile(err)
	}

	return string(data), nil
}

// Gets the latest stable release tags from github for a given org name and repo name(in that org) in sorted order
func GetLatestReleaseTagsSorted(org string, repo string) ([]string, error) {
	var url string = "https://github.com/" + org + "/" + repo + "/releases"
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

type Commit struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

type ReleaseTags struct {
	Name string `json:"name"`
	Commit Commit `json:"commit"`
	TarballURL string `json:"tarball_url"`
	ZipballURL string `json:"zipball_url"`
	NodeID string `json:"node_id"`
}

// Gets release tag from github for a given org name, repo name(in that org) and tag 
func GetLatestReleaseTagCommitSHA(org string, repo string) (string, error) {
    url := fmt.Sprintf("https://github.com/%s/%s/tags", org, repo)

    resp, err := http.Get(url)
    if err != nil {
        return "", errors.New("cannot get list of tags from Github")
    }
    defer safeClose(resp.Body)

	if (resp.StatusCode == 404) {
		return "", errors.New("repository is not found")
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile("/commit/(.*?)\"")
	releases := re.FindAllString(string(body), -1)
	if len(releases) == 0 {
		return "", errors.New("no commit found in this repository")
	}
	var commits []string
	for _, rel := range releases {
		latest := strings.ReplaceAll(rel, "/commit/", "")
		latest = strings.ReplaceAll(latest, "\"", "")
		commits = append(commits, latest)
	}

	return commits[0], nil
}

// SafeClose is a helper function help to close the io
func safeClose(co io.Closer) {
	if cerr := co.Close(); cerr != nil {
		log.Error(cerr)
	}
}

func Contains[G []K, K comparable](slice G, ele K) bool {
	for _, item := range slice {
		if item == ele {
			return true
		}
	}
	return false
}

func Cast[K any](val interface{}) (K, error) {
	var assertedValue K
	if IsInterfaceNil(val) {
		return assertedValue, ErrTypeCast(fmt.Errorf("nil interface cannot be type casted"))
	}
	var ok bool
	assertedValue, ok = val.(K)
	if !ok {
		return assertedValue, ErrTypeCast(fmt.Errorf("the underlying type of the interface is %s", reflect.TypeOf(val).Name()))
	}
	return assertedValue, nil
}

func MarshalAndUnmarshal[fromType any, toType any](val fromType) (unmarshalledvalue toType, err error) {
	data, err := Marshal(val)
	if err != nil {
		return
	}

	err = Unmarshal(data, &unmarshalledvalue)
	if err != nil {
		return
	}
	return
}

func IsClosed[K any](ch chan K) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// WriteToFile writes the given content to the given file path
func WriteToFile(path string, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return ErrCreateFile(err, path)
	}

	_, err = file.WriteString(content)
	if err != nil {
		return ErrWriteFile(err, path)
	}
	// Close the file to save the changes.
	err = file.Close()
	if err != nil {
		return ErrWriteFile(err, path)
	}
	return nil
}

// FormatName formats the given string to by replacing " " with "-"
func FormatName(input string) string {
	formatedName := strings.ReplaceAll(input, " ", "-")
	formatedName = strings.ToLower(formatedName)
	return formatedName
}

func GetRandomAlphabetsOfDigit(length int) (s string) {
	charSet := "abcdedfghijklmnopqrstuvwxyz"
	for i := 0; i < length; i++ {
		random := mathrand.Intn(len(charSet))
		randomChar := charSet[random]
		s += string(randomChar)
	}
	return
}

// combineErrors merges a slice of error
// into one error separated by the given separator
func CombineErrors(errs []error, sep string) error {
	if len(errs) == 0 {
		return nil
	}

	var errString []string
	for _, err := range errs {
		errString = append(errString, err.Error())
	}

	return errors.New(strings.Join(errString, sep))
}

func MergeMaps(mergeInto, toMerge map[string]interface{}) map[string]interface{} {
	if mergeInto == nil {
		mergeInto = make(map[string]interface{})
	}
	for k, v := range toMerge {
		mergeInto[k] = v
	}
	return mergeInto
}

func WriteYamlToFile[K any](outputPath string, data K) error {
	byt, err := yaml.Marshal(data)
	if err != nil {
		// Use a different error code
		return ErrMarshal(err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return ErrCreateFile(err, outputPath)
	}

	_, err = file.Write(byt)
	if err != nil {
		return ErrWriteFile(err, outputPath)
	}
	return nil
}

func WriteJSONToFile[K any](outputPath string, data K) error {
	byt, err := json.MarshalIndent(data, " ", " ")
	if err != nil {
		return ErrMarshal(err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return ErrCreateFile(err, outputPath)
	}

	_, err = file.Write(byt)
	if err != nil {
		return ErrWriteFile(err, outputPath)
	}
	return nil
}

func CreateDirectory(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		err = ErrCreateDir(err, path)

		return err
	}
	return nil
}

func ReplaceSpacesAndConvertToLowercase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}
func ReplaceSpacesWithHyphenAndConvertToLowercase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}
func ExtractDomainFromURL(location string) string {
	parsedURL, err := url.Parse(location)
	// If unable to extract domain return the location as is.
	if err != nil {
		return location
	}
	return regexp.MustCompile(`(([a-zA-Z0-9]+\.)([a-zA-Z0-9]+))$`).FindString(parsedURL.Hostname())
}

func IsInterfaceNil(val interface{}) bool {
	if val == nil {
		return true
	}
	return reflect.ValueOf(val).IsZero()
}

func IsSchemaEmpty(schema string) (valid bool) {
	if schema == "" {
		return
	}
	m := make(map[string]interface{})
	_ = json.Unmarshal([]byte(schema), &m)
	if m["properties"] == nil {
		return
	}
	valid = true
	return
}
func FindEntityType(content []byte) (entity.EntityType, error) {
	var tempMap map[string]interface{}
	if err := json.Unmarshal(content, &tempMap); err != nil {
		return "", ErrUnmarshal(err)
	}
	schemaVersion, err := Cast[string](tempMap["schemaVersion"])
	if err != nil {
		return "", ErrInvalidSchemaVersion
	}
	lastIndex := strings.LastIndex(schemaVersion, "/")
	if lastIndex != -1 {
		schemaVersion = schemaVersion[:lastIndex]
	}
	switch schemaVersion {
	case "relationships.meshery.io":
		return entity.RelationshipDefinition, nil
	case "components.meshery.io":
		return entity.ComponentDefinition, nil
	case "models.meshery.io":
		return entity.Model, nil
	case "policies.meshery.io":
		return entity.PolicyDefinition, nil
	}
	return "", ErrInvalidSchemaVersion
}

// RecursiveCastMapStringInterfaceToMapStringInterface will convert a
// map[string]interface{} recursively => map[string]interface{}
func RecursiveCastMapStringInterfaceToMapStringInterface(in map[string]interface{}) map[string]interface{} {
	res := ConvertMapInterfaceMapString(in)
	out, ok := res.(map[string]interface{})
	if !ok {
		fmt.Println("failed to cast")
	}

	return out
}

// ConvertMapInterfaceMapString converts map[interface{}]interface{} => map[string]interface{}
//
// It will also convert []interface{} => []string
func ConvertMapInterfaceMapString(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, v2 := range x {
			switch k2 := k.(type) {
			case string:
				m[k2] = ConvertMapInterfaceMapString(v2)
			default:
				m[fmt.Sprint(k)] = ConvertMapInterfaceMapString(v2)
			}
		}
		v = m

	case []interface{}:
		for i, v2 := range x {
			x[i] = ConvertMapInterfaceMapString(v2)
		}

	case map[string]interface{}:
		for k, v2 := range x {
			x[k] = ConvertMapInterfaceMapString(v2)
		}
	}

	return v
}
func ConvertToJSONCompatible(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for key, value := range v {
			m[key.(string)] = ConvertToJSONCompatible(value)
		}
		return m
	case []interface{}:
		for i, item := range v {
			v[i] = ConvertToJSONCompatible(item)
		}
	}
	return data
}
func YAMLToJSON(content []byte) ([]byte, error) {
	var jsonData interface{}
	if err := yaml.Unmarshal(content, &jsonData); err == nil {
		jsonData = ConvertToJSONCompatible(jsonData)
		convertedContent, err := json.Marshal(jsonData)
		if err == nil {
			content = convertedContent
		} else {
			return nil, ErrUnmarshal(err)
		}
	} else {
		return nil, ErrUnmarshal(err)
	}
	return content, nil
}
func ExtractFile(filePath string, destDir string) error {
	if IsTarGz(filePath) {
		return ExtractTarGz(destDir, filePath)
	} else if IsZip(filePath) {
		return ExtractZip(destDir, filePath)
	}
	return ErrExtractType
}

// Convert path to svg Data
func ReadSVGData(baseDir, path string) (string, error) {
	fullPath := baseDir + path
	svgData, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(svgData), nil
}
func Compress(src string, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	defer zr.Close()
	tw := tar.NewWriter(zr)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			_, err = io.Copy(tw, data)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Check if a string is purely numeric
func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Split version into components (numeric and non-numeric) using both '.' and '-'
func splitVersion(version string) []string {
	version = strings.ReplaceAll(version, "-", ".")
	return strings.Split(version, ".")
}

// Compare two version strings
func compareVersions(v1, v2 string) int {
	v1Components := splitVersion(v1)
	v2Components := splitVersion(v2)

	maxLen := len(v1Components)
	if len(v2Components) > maxLen {
		maxLen = len(v2Components)
	}

	for i := 0; i < maxLen; i++ {
		var part1, part2 string
		if i < len(v1Components) {
			part1 = v1Components[i]
		}
		if i < len(v2Components) {
			part2 = v2Components[i]
		}

		if isNumeric(part1) && isNumeric(part2) {
			num1, _ := strconv.Atoi(part1)
			num2, _ := strconv.Atoi(part2)
			if num1 != num2 {
				return num1 - num2
			}
		} else if isNumeric(part1) && !isNumeric(part2) {
			return -1
		} else if !isNumeric(part1) && isNumeric(part2) {
			return 1
		} else {
			if part1 != part2 {
				return strings.Compare(part1, part2)
			}
		}
	}

	return 0
}

// Function to get all version directories sorted in descending order
func GetAllVersionDirsSortedDesc(modelVersionsDirPath string) ([]string, error) {
	type versionInfo struct {
		original string
		dirPath  string
	}
	entries, err := os.ReadDir(modelVersionsDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory '%s': %w", modelVersionsDirPath, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no version directories found in '%s'", modelVersionsDirPath)
	}

	versions := []versionInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		versionDirPath := filepath.Join(modelVersionsDirPath, entry.Name())
		versionStr := entry.Name()

		// Optionally remove leading 'v'
		versionStr = strings.TrimPrefix(versionStr, "v")

		if versionStr == "" {
			continue
		}

		versions = append(versions, versionInfo{
			original: versionStr,
			dirPath:  versionDirPath,
		})
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no valid version directories found in '%s'", modelVersionsDirPath)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].original, versions[j].original) > 0
	})

	sortedDirPaths := make([]string, len(versions))
	for i, v := range versions {
		sortedDirPaths[i] = v.dirPath
	}

	return sortedDirPaths, nil
}

// isDirectoryNonEmpty checks if a directory exists and is non-empty
func IsDirectoryNonEmpty(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	if !fi.IsDir() {
		return false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	return len(entries) > 0
}
