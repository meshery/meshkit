package registration

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/utils/store"
)

var hashCheckSVG = store.NewGenericThreadSafeStore[string]() // Using the generic store
var UISVGPaths = make([]string, 1)

func WriteAndReplaceSVGWithFileSystemPath(svgColor, svgWhite, svgComplete string, baseDir, dirname, filename string) (svgColorPath, svgWhitePath, svgCompletePath string) {
	filename = strings.ToLower(filename)
	successCreatingDirectory := false
	defer func() {
		if successCreatingDirectory {
			UISVGPaths = append(UISVGPaths, filepath.Join(baseDir, dirname))
		}
	}()
	if svgColor != "" {
		path := filepath.Join(baseDir, dirname, "color")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		hash := md5.Sum([]byte(svgColor))
		hashString := hex.EncodeToString(hash[:])
		if pathsvg, exists := hashCheckSVG.Get(hashString); exists { // the image has already been loaded, point the component to that path
			svgColorPath = pathsvg
			goto White
		}
		f, err := os.Create(filepath.Join(path, filename+"-color.svg"))
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = f.WriteString(svgColor)
		if err != nil {
			fmt.Println(err)
			return
		}
		svgColorPath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "color", filename+"-color.svg"))
		hashCheckSVG.Set(hashString, svgColorPath)
	}
White:
	if svgWhite != "" {
		path := filepath.Join(baseDir, dirname, "white")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		hash := md5.Sum([]byte(svgWhite))
		hashString := hex.EncodeToString(hash[:])
		if pathsvg, exists := hashCheckSVG.Get(hashString); exists { // the image has already been loaded, point the component to that path
			svgWhitePath = pathsvg
			goto Complete
		}
		f, err := os.Create(filepath.Join(path, filename+"-white.svg"))
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = f.WriteString(svgWhite)
		if err != nil {
			fmt.Println(err)
			return
		}
		svgWhitePath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "white", filename+"-white.svg"))
		hashCheckSVG.Set(hashString, svgWhitePath)
	}
Complete:
	if svgComplete != "" {
		path := filepath.Join(baseDir, dirname, "complete")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		hash := md5.Sum([]byte(svgComplete))
		hashString := hex.EncodeToString(hash[:])
		if pathsvg, exists := hashCheckSVG.Get(hashString); exists { // the image has already been loaded, point the component to that path
			svgCompletePath = pathsvg
			return
		}
		f, err := os.Create(filepath.Join(path, filename+"-complete.svg"))
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = f.WriteString(svgComplete)
		if err != nil {
			fmt.Println(err)
			return
		}
		svgCompletePath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "complete", filename+"-complete.svg"))
		hashCheckSVG.Set(hashString, svgCompletePath)
	}
	return
}

// func WriteAndReplaceSVGWithFileSystemPath(metadata map[string]interface{}, baseDir, dirname, filename string) {

func getRelativePathForAPI(baseDir, path string) string {
	ui := strings.TrimPrefix(baseDir, "../../")
	return filepath.Join(ui, path)
}
