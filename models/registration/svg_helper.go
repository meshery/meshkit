package registration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var UISVGPaths = make([]string, 1)

type svgFile interface {
	WriteString(string) (int, error)
	Close() error
}

var createSVGFile = func(path string) (svgFile, error) {
	return os.Create(path)
}

func writeAndCloseSVG(path, content string) error {
	f, err := createSVGFile(path)
	if err != nil {
		return err
	}

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
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

		if err := writeAndCloseSVG(
			filepath.Join(path, filename+"-color.svg"),
			svgColor,
		); err != nil {
			fmt.Println(err)
			svgColorPath, svgWhitePath, svgCompletePath = "", "", ""
			return
		}

		svgColorPath = getRelativePathForAPI(
			baseDir,
			filepath.Join(dirname, "color", filename+"-color.svg"),
		)
	}

	if svgWhite != "" {
		path := filepath.Join(baseDir, dirname, "white")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		if err := writeAndCloseSVG(
			filepath.Join(path, filename+"-white.svg"),
			svgWhite,
		); err != nil {
			fmt.Println(err)
			svgColorPath, svgWhitePath, svgCompletePath = "", "", ""
			return
		}

		svgWhitePath = getRelativePathForAPI(
			baseDir,
			filepath.Join(dirname, "white", filename+"-white.svg"),
		)
	}

	if svgComplete != "" {
		path := filepath.Join(baseDir, dirname, "complete")
		err := os.MkdirAll(path, 0777)
		if err != nil {
			fmt.Println(err)
			return
		}
		successCreatingDirectory = true

		if err := writeAndCloseSVG(
			filepath.Join(path, filename+"-complete.svg"),
			svgComplete,
		); err != nil {
			fmt.Println(err)
			svgColorPath, svgWhitePath, svgCompletePath = "", "", ""
			return
		}

		svgCompletePath = getRelativePathForAPI(
			baseDir,
			filepath.Join(dirname, "complete", filename+"-complete.svg"),
		)
	}

	return
}

func getRelativePathForAPI(baseDir, path string) string {
	ui := strings.TrimPrefix(baseDir, "../../")
	return filepath.Join(ui, path)
}
