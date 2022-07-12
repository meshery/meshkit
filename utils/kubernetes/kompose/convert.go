package kompose

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/kubernetes/kompose/pkg/app"
	"github.com/kubernetes/kompose/pkg/kobject"
	"github.com/layer5io/meshkit/utils"
	"gopkg.in/yaml.v2"
)

var (
	list = "List"
)

const DefaultDockerComposeSchemaURL = "https://raw.githubusercontent.com/compose-spec/compose-spec/master/schema/compose-spec.json"

// Checks whether the given manifest is a valid docker-compose file.
// schemaURL is assigned a default url if not specified
// error will be 'nil' if it is a valid docker compose file
func IsManifestADockerCompose(manifest []byte, schemaURL string) error {
	if schemaURL == "" {
		schemaURL = DefaultDockerComposeSchemaURL
	}
	schema, err := utils.ReadRemoteFile(schemaURL)
	if err != nil {
		return err
	}
	var dockerComposeFile DockerComposeFile
	dockerComposeFile = manifest
	err = dockerComposeFile.Validate([]byte(schema))
	return err
}

// converts a given docker-compose file into kubernetes manifests
// expects a validated docker-compose file
func Convert(dockerCompose DockerComposeFile) (string, error) {
	err := utils.CreateFile(dockerCompose, "temp.data", "./")
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	defer func() {
		os.Remove("temp.data")
		os.Remove("result.yaml")
	}()

<<<<<<< HEAD
	err = formatComposeFile(&dockerCompose)
=======
	formatComposeFile(&dockerCompose)
>>>>>>> upstream/master
	if err != nil {
		return "", ErrCvrtKompose(err)
	}
	err = versionCheck(dockerCompose)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

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

type composeFile struct {
	Version string `yaml:"version,omitempty"`
}

// checks if the version is compatible with `kompose`
// expects a valid docker compose yaml
// error = nil means it is compatible
func versionCheck(dc DockerComposeFile) error {
	cf := composeFile{}
	err := yaml.Unmarshal(dc, &cf)
	if err != nil {
		return utils.ErrUnmarshal(err)
	}
	if cf.Version == "" {
		return ErrNoVersion()
	}
	versionFloatVal, err := strconv.ParseFloat(cf.Version, 64)
	if err != nil {
		return utils.ErrExpectedTypeMismatch(err, "float")
	} else {
		if versionFloatVal > 3.3 {
			// kompose throws a fatal error when version exceeds 3.3
			// need this till this PR gets merged https://github.com/kubernetes/kompose/pull/1440(move away from libcompose to compose-go)
			return ErrIncompatibleVersion()
		}
	}
	return nil
}

// formatComposeFile takes in a pointer to the compose file byte array and formats it so that it is compatible with `Kompose`
// it expects a validated docker compose file and does not validate
func formatComposeFile(yamlManifest *DockerComposeFile) {
	data := composeFile{}
	err := yaml.Unmarshal(*yamlManifest, &data)
	if err != nil {
		return
	}
	// so that "3.3" and 3.3 are treated differently by `Kompose`
	data.Version = fmt.Sprintf("%s", data.Version)
	out, err := yaml.Marshal(data)
	if err != nil {
		return
	}
	*yamlManifest = out
	return
}
