package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
			expectedData, _ := ioutil.ReadFile(tt.filepath)
			if string(gotData) != string(expectedData) {
				t.Errorf(fmt.Sprintf("expected %s, \ngot %s", string(expectedData), string(gotData)))
			}
		}
	}
}
