package github

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestGetPackage(t *testing.T) {
	var tests = []struct {
		gh       GithubReleasePackage
		filepath string
	}{
		{
			GithubReleasePackage{
				Owner:      "vmware-tanzu",
				Repository: "helm-charts",
				Version:    "velero-2.32.1",
			},
			"./components_valero.json"},
	}
	for _, tt := range tests {
		got, err := tt.gh.GetPackage()
		if err != nil {
			t.Errorf(err.Error())
			return
		}
		comp, err := got.GenerateComponents()
		if err != nil {
			t.Errorf(err.Error())
			return
		}

		if len(comp) != 0 {
			gotData, _ := json.Marshal(comp)
			file, err := os.Open(tt.filepath)
			if err != nil {
				t.Error(err)
			}
			buf, err := io.ReadAll(file)
			if err != nil {
				t.Error(err)
			}
			if string(gotData) != string(buf) {
				t.Errorf(fmt.Sprintf("expected %s, \ngot %s", string(buf), string(gotData)))
			}
		}
	}
}
