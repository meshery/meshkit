package kompose

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/kubernetes/kompose/pkg/app"
	"github.com/kubernetes/kompose/pkg/kobject"
	"gopkg.in/yaml.v2"
)

var (
	list = "List"
)

// converts a given docker-compose file into kubernetes manifests
func Convert(dockerCompose string) (string, error) {
	err := ioutil.WriteFile("temp.data", []byte(dockerCompose), 0666)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	defer func() {
		os.Remove("temp.data")
		os.Remove("result.yaml")
	}()

	ConvertOpt := kobject.ConvertOptions{
		ToStdout:     false,
		CreateChart:  false, // for helm charts
		GenerateYaml: true,
		GenerateJSON: false,
		Replicas:     1,
		InputFiles:   []string{"temp.data"},
		OutFile:      "result.yaml",
		Provider:     "kubernetes",
		CreateD:      false,
		CreateDS:     false, CreateRC: false,
		Build:                       "none",
		BuildRepo:                   "",
		BuildBranch:                 "",
		PushImage:                   false,
		PushImageRegistry:           "",
		CreateDeploymentConfig:      true,
		EmptyVols:                   false,
		Volumes:                     "persistentVolumeClaim",
		PVCRequestSize:              "",
		InsecureRepository:          false,
		IsDeploymentFlag:            false,
		IsDaemonSetFlag:             false,
		IsReplicationControllerFlag: false,
		Controller:                  "",
		IsReplicaSetFlag:            false,
		IsDeploymentConfigFlag:      false,
		YAMLIndent:                  2,
		WithKomposeAnnotation:       true,
		MultipleContainerMode:       false,
		ServiceGroupMode:            "",
		ServiceGroupName:            "",
	}
	app.Convert(ConvertOpt)

	result, err := ioutil.ReadFile("result.yaml")
	if err != nil {
		return "", ErrCvrtKompose(err)
	}
	formattedResult, err := formatConvertedManifest(string(result))
	if err != nil {
		return "", ErrCvrtKompose(err)
	}
	return formattedResult, nil
}

func formatConvertedManifest(k8sMan string) (string, error) {
	formattedManifest := ""

	manifest := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(k8sMan), &manifest); err != nil {
		return "", err
	}

	if manifest["kind"] == list {
		items := manifest["items"].([]interface{})
		tempMans := []string{}
		for _, resMan := range items {
			res, err := yaml.Marshal(&resMan)
			if err != nil {
				return formattedManifest, nil
			}
			tempMans = append(tempMans, string(res))
		}
		formattedManifest = strings.Join(tempMans, "\n---\n")
	}
	return formattedManifest, nil
}
