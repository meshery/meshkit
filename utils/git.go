package utils

import (
	"bytes"
	"encoding/csv"
	"io/ioutil"
	"strings"
)

const (
	ok          = "ok"
	unavailable = "unavailable"
)

var (
	// gitVersionFilePath determines git generated version path
	gitVersionFilePath = "./version"
)

// git method which allows fetch the git HEAD tag version and commit number
func Git() (version, commitHead string) {

	b, _ := ioutil.ReadFile(gitVersionFilePath)
	if b != nil {
		reader := bytes.NewReader(b)
		r := csv.NewReader(reader)
		rows, _ := r.ReadAll()
		for idx, row := range rows {
			if len(row) < 1 {
				continue
			}
			switch idx {
			case 0:
				{
					commitHead = strings.TrimSpace(row[0])
				}
			case 1:
				{
					version = strings.TrimSpace(row[0])
					break
				}
			}
		}
	}
	return
}
