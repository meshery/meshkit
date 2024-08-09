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

func WriteAndReplaceSVGWithFileSystemPath(metadata map[string]interface{}, baseDir, dirname, filename string) {
	filename = strings.ToLower(filename)
	successCreatingDirectory := false
	defer func() {
		if successCreatingDirectory {
			UISVGPaths = append(UISVGPaths, filepath.Join(baseDir, dirname))
		}
	}()
	if metadata["svgColor"] != "" {
		path := filepath.Join(baseDir, dirname, "color")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		x, ok := metadata["svgColor"].(string)
		if ok {
			hash := md5.Sum([]byte(x))
			hashString := hex.EncodeToString(hash[:])
			pathsvg := hashCheckSVG[hashString]
			if pathsvg != "" { // the image has already been loaded, point the component to that path
				metadata["svgColor"] = pathsvg
				goto White
			}
			f, err := os.Create(filepath.Join(path, filename+"-color.svg"))
			if err != nil {
				fmt.Println(err)
				return
			}
			_, err = f.WriteString(x)
			if err != nil {
				fmt.Println(err)
				return
			}
			metadata["svgColor"] = getRelativePathForAPI(baseDir, filepath.Join(dirname, "color", filename+"-color.svg")) //Replace the actual SVG with path to SVG
			writeHashCheckSVG(hashString, metadata["svgColor"].(string))
		}
	}
White:
	if metadata["svgWhite"] != "" {
		path := filepath.Join(baseDir, dirname, "white")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		x, ok := metadata["svgWhite"].(string)
		if ok {
			hash := md5.Sum([]byte(x))
			hashString := hex.EncodeToString(hash[:])
			pathsvg := hashCheckSVG[hashString]
			if pathsvg != "" { // the image has already been loaded, point the component to that path
				metadata["svgWhite"] = pathsvg
				goto Complete
			}
			f, err := os.Create(filepath.Join(path, filename+"-white.svg"))
			if err != nil {
				fmt.Println(err)
				return
			}
			_, err = f.WriteString(x)
			if err != nil {
				fmt.Println(err)
				return
			}
			metadata["svgWhite"] = getRelativePathForAPI(baseDir, filepath.Join(dirname, "white", filename+"-white.svg")) //Replace the actual SVG with path to SVG
			writeHashCheckSVG(hashString, metadata["svgWhite"].(string))
		}
	}
Complete:
	if metadata["svgComplete"] != "" {
		path := filepath.Join(baseDir, dirname, "complete")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		x, ok := metadata["svgComplete"].(string)
		if ok {
			hash := md5.Sum([]byte(x))
			hashString := hex.EncodeToString(hash[:])
			pathsvg := hashCheckSVG[hashString]
			if pathsvg != "" { // the image has already been loaded, point the component to that path
				metadata["svgComplete"] = pathsvg
				return
			}
			f, err := os.Create(filepath.Join(path, filename+"-complete.svg"))
			if err != nil {
				fmt.Println(err)
				return
			}
			_, err = f.WriteString(x)
			if err != nil {
				fmt.Println(err)
				return
			}
			metadata["svgComplete"] = getRelativePathForAPI(baseDir, filepath.Join(dirname, "complete", filename+"-complete.svg")) //Replace the actual SVG with path to SVG
			writeHashCheckSVG(hashString, metadata["svgComplete"].(string))
		}
	}
}

func getRelativePathForAPI(baseDir, path string) string {
	ui := strings.TrimPrefix(baseDir, "../../")
	return filepath.Join(ui, path)
}
