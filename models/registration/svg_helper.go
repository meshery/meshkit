package registration

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var hashCheckSVG = make(map[string]string)
var mx sync.Mutex
var UISVGPaths = make([]string, 1)

func writeHashCheckSVG(key string, val string) {
	mx.Lock()
	hashCheckSVG[key] = val
	mx.Unlock()
}

func WriteAndReplaceSVGWithFileSystemPath(svgColor, svgWhite, svgComplete string, baseDir, dirname, filename string, isModel bool) (svgColorPath, svgWhitePath, svgCompletePath string) {
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
		pathsvg := hashCheckSVG[hashString]
		if pathsvg != "" && !isModel { // the image has already been loaded, point the component to that path
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
		svgColorPath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "color", filename+"-color.svg")) //Replace the actual SVG with path to SVG
		writeHashCheckSVG(hashString, svgColorPath)

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
		pathsvg := hashCheckSVG[hashString]
		if pathsvg != "" && !isModel { // the image has already been loaded, point the component to that path
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
		svgWhitePath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "white", filename+"-white.svg")) //Replace the actual SVG with path to SVG
		writeHashCheckSVG(hashString, svgWhitePath)

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
		pathsvg := hashCheckSVG[hashString]
		if pathsvg != "" && !isModel { // the image has already been loaded, point the component to that path
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
		svgCompletePath = getRelativePathForAPI(baseDir, filepath.Join(dirname, "complete", filename+"-complete.svg")) //Replace the actual SVG with path to SVG
		writeHashCheckSVG(hashString, svgCompletePath)

	}
	return
}

func getRelativePathForAPI(baseDir, path string) string {
	ui := strings.TrimPrefix(baseDir, "../../")
	return filepath.Join(ui, path)
}
