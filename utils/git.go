package utils

import (
	"bytes"
	"encoding/csv"
	"os"
	"strings"
)

var (
	// gitVersionFilePath determines git generated version path
	gitVersionFilePath = "./version"
)

// git method which allows fetch the git HEAD tag version and commit number
func Git() (version, commitHead string) {
	b, _ := os.ReadFile(gitVersionFilePath)
	if b != nil {
		reader := bytes.NewReader(b)
		r := csv.NewReader(reader)
		rows, _ := r.ReadAll()
		fieldIndex := 0
		for _, row := range rows {
			if len(row) < 1 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
				continue
			}
			switch fieldIndex {
			case 0:
				{
					commitHead = strings.TrimSpace(row[0])
					fieldIndex++
				}
			case 1:
				{
					version = strings.TrimSpace(row[0])
					return
				}
			}
		}
	}
	return
}
