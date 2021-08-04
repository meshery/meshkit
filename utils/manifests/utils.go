package manifests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
)

func getDefinitions(template string, crd string, resource int, cfg Config) (string, error) {
	var def v1alpha1.WorkloadDefinition
	definitionRef := strings.ToLower(crd) + ".meshery.layer5.io"

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
				"k8sAPIVersion": "",
				"k8skind":       "",
			}
		}
	case K8:
		{
			def.Spec.Metadata = map[string]string{
				"@type":         "pattern.meshery.io/k8s",
				"k8sAPIVersion": "",
				"k8skind":       "",
			}
		}
	case MESHERY:
		{
			def.Spec.Metadata = map[string]string{
				"@type":         "pattern.meshery.io/core",
				"k8sAPIVersion": "",
				"k8skind":       "",
			}
		}
	}
	out, err := json.MarshalIndent(def, "", " ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getSchema(crd string, filepath string, binPath string) (string, error) {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	crdname := strings.ToLower(crd)
	getSchemaCmdArgs := []string{"--location", filepath, "-t", "yaml", "--filter", "$[?(@.kind==\"CustomResourceDefinition\" && @.spec.names.kind=='" + crd + "')]..openAPIV3Schema.properties.spec", " --o-filter", "$[]", "-o", "json"}
	schemaCmd := exec.Command(binPath, getSchemaCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er
	err := schemaCmd.Run()
	if err != nil {
		return "", err
	}
	schema := []map[string]interface{}{}
	if err := json.Unmarshal([]byte(out.String()), &schema); err != nil {
		return "", err
	}
	schema[0]["title"] = crdname
	var output []byte
	output, err = json.MarshalIndent(schema[0], "", " ")
	if err != nil {
		fmt.Println("ERR " + err.Error())
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
	return crds[1 : len(crds)-2] // first and last characters are "[" and "]" respectively
}
