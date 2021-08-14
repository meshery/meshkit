package manifests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
)

func getDefinitions(template string, crd string, resource int, cfg Config, filepath string, binPath string) (string, error) {
	var def v1alpha1.WorkloadDefinition
	definitionRef := strings.ToLower(crd) + ".meshery.layer5.io"
	apiVersion, err := getApiVersion(binPath, filepath, crd)
	if err != nil {
		return "", err
	}
	apiGroup, err := getApiGrp(binPath, filepath, crd)
	if err != nil {
		return "", err
	}
	//getting defintions for different native resources
	def.Spec.DefinitionRef.Name = definitionRef
	def.ObjectMeta.Name = crd
	switch resource {
	case SERVICE_MESH:
		{
			def.Spec.Metadata = map[string]string{
				"@type":         "pattern.meshery.io/mesh/workload",
				"meshVersion":   cfg.MeshVersion,
				"meshName":      cfg.Name,
				"k8sAPIVersion": apiGroup + "/" + apiVersion,
				"k8skind":       crd,
			}
		}
	case K8s:
		{
			def.Spec.Metadata = map[string]string{
				"@type":         "pattern.meshery.io/k8s",
				"k8sAPIVersion": apiGroup + "/" + apiVersion,
				"k8skind":       "",
			}
		}
	case MESHERY:
		{
			def.Spec.Metadata = map[string]string{
				"@type": "pattern.meshery.io/core",
			}
		}
	}
	out, err := json.MarshalIndent(def, "", " ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getSchema(crd string, filepath string, binPath string, cfg Config) (string, error) {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	crdname := strings.ToLower(crd)
	filter := []string{"$..openAPIV3Schema.properties.spec", " --o-filter", "$[]", "-o", "json"} //cfg.Filter.Spec
	getSchemaCmdArgs := []string{"--location", filepath, "-t", "yaml", "--filter", "$[?(@.spec.names.kind=='" + crd + "')]", "--filter"}
	getSchemaCmdArgs = append(getSchemaCmdArgs, filter...)
	schemaCmd := exec.Command(binPath, getSchemaCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er
	err := schemaCmd.Run()
	if err != nil {
		return "", err
	}
	schema := [][]map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &schema); err != nil {
		return "", err
	}
	(schema)[0][0]["title"] = crdname
	var output []byte
	output, err = json.MarshalIndent(schema[0][0], "", " ")
	if err != nil {
		return "", err
	}
	return string(output), nil
}
func populateTempyaml(yaml string, path string) error {
	var _, err = os.Stat(path)
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	//delete any previous contents from the file
	if err := os.Truncate(path, 0); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(yaml)
	if err != nil {
		return err
	}
	err = file.Sync()
	if err != nil {
		return err
	}

	return nil
}

func getCrdnames(s string) []string {
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "")
	crds := strings.Split(s, "\n")
	if len(crds) <= 2 {
		return []string{}
	}
	return crds[1 : len(crds)-2] // first and last characters are "[" and "]" respectively
}

func getApiVersion(binPath string, filepath string, crd string) (string, error) {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	filter := []string{"$..spec.versions[0]", " --o-filter", "$[0]"} //cfg.Filter.VersionFilter
	filter = append(filter, "-o", "json")
	getAPIvCmdArgs := []string{"--location", filepath, "-t", "yaml", "--filter", "$[?(@.spec.names.kind=='" + crd + "')]", "--filter"}
	getAPIvCmdArgs = append(getAPIvCmdArgs, filter...)
	schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er
	err := schemaCmd.Run()
	if err != nil {
		return er.String(), ErrGetAPIVersion(err)
	}
	grp := [][]map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &grp); err != nil {
		return "", err
	}
	var output []byte
	output, err = json.Marshal(grp[0][0]["name"])
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(output), "\"", "")
	return s, nil
}
func getApiGrp(binPath string, filepath string, crd string) (string, error) {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	filter := []string{"$..spec", " --o-filter", "$[]"}
	filter = append(filter, "-o", "json")
	getAPIvCmdArgs := []string{"--location", filepath, "-t", "yaml", "--filter", "$[?(@.spec.names.kind=='" + crd + "')]", "--filter"}
	getAPIvCmdArgs = append(getAPIvCmdArgs, filter...)
	schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er
	err := schemaCmd.Run()
	if err != nil {
		return er.String(), ErrGetAPIGroup(err)
	}
	v := [][]map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		return "", err
	}
	var output []byte
	output, err = json.Marshal(v[0][0]["group"])
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(output), "\"", "")
	return s, nil
}

func filterYaml(yamlPath string, filter []string, binPath string) error {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	filter = append(filter, "-o", "yaml")
	getCrdsCmdArgs := append([]string{"--location", yamlPath, "-t", "yaml", "--filter"}, filter...)
	cmd := exec.Command(binPath, getCrdsCmdArgs...)
	//emptying buffers
	out.Reset()
	er.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	if err != nil {
		return ErrGetCrdNames(err)
	}
	path := filepath.Join(os.TempDir(), "/test.yaml")
	err = populateTempyaml(out.String(), path)
	if err != nil {
		return ErrPopulatingYaml(err)
	}
	return nil
}

//cleanup
func deleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}
