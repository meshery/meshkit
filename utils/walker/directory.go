package walker

import (
	"io"
	"os"
	"path/filepath"
)

func WalkLocalDirectory(path string) ([]*File, error) {
	var files []*File
	err := filepath.WalkDir(path,
		func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				var f *File
				file, err := os.OpenFile(path, os.O_RDONLY, 0444)
				if err != nil {
					return err
				}
				content, err := io.ReadAll(file)
				if err != nil {
					return err
				}

				f.Name = d.Name()
				f.Path = path
				f.Content = string(content)
				files = append(files, f)
			}
			return nil
		})

	if err != nil {
		return nil, err
	}

	return files, nil

}
