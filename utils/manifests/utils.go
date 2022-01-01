package manifests

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
)

func getDefinitions(crd string, resource int, cfg Config, filepath string, binPath string) (string, error) {
	//the default input format is "yaml"
	inputFormat := "yaml"
	if cfg.Filter.IsJson {
		inputFormat = "json"
	}
	var def v1alpha1.WorkloadDefinition
	definitionRef := strings.ToLower(crd) + ".meshery.layer5.io"
	apiVersion, err := getApiVersion(binPath, filepath, crd, inputFormat, cfg)
	if err != nil {
		return "", ErrGetAPIVersion(err)
	}
	apiGroup, err := getApiGrp(binPath, filepath, crd, inputFormat, cfg)
	if err != nil {
		return "", ErrGetAPIGroup(err)
	}
	//getting defintions for different native resources
	def.Spec.DefinitionRef.Name = definitionRef
	def.ObjectMeta.Name = crd
	def.APIVersion = "core.oam.dev/v1alpha1"
	def.Kind = "WorkloadDefinition"
	switch resource {
	case SERVICE_MESH:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/mesh/workload",
			"meshVersion":   cfg.MeshVersion,
			"meshName":      cfg.Name,
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       crd,
		}
	case K8s:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/k8s",
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       crd,
			"version":       cfg.K8sVersion,
		}
		def.ObjectMeta.Name += ".K8s"
		def.Spec.DefinitionRef.Name = strings.ToLower(crd) + ".k8s.meshery.layer5.io"
	case MESHERY:
		def.Spec.Metadata = map[string]string{
			"@type": "pattern.meshery.io/core",
		}
	}
	out, err := json.MarshalIndent(def, "", " ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getSchema(crd string, fp string, binPath string, cfg Config) (string, error) {
	//few checks to avoid index out of bound panic
	if len(cfg.Filter.ItrSpecFilter) == 0 {
		return "", ErrAbsentFilter(errors.New("Empty ItrSpecFilter"))
	}
	inputFormat := "yaml"
	if cfg.Filter.IsJson {
		inputFormat = "json"
	}

	crdname := strings.ToLower(crd)
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	if len(cfg.Filter.SpecFilter) != 0 { //If SpecFilter is passed then it will evaluated in output filter. [currently this case is for service meshes]
		itr := cfg.Filter.ItrSpecFilter[0] + "=='" + crd + "')]"
		for _, f := range cfg.Filter.ItrSpecFilter[1:] {
			itr += f
		}
		getAPIvCmdArgs := []string{"--location", fp, "-t", inputFormat, "--filter", itr, "--o-filter"}
		getAPIvCmdArgs = append(getAPIvCmdArgs, cfg.Filter.SpecFilter...)
		schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
		schemaCmd.Stdout = &out
		schemaCmd.Stderr = &er
		err := schemaCmd.Run()
		if err != nil {
			return er.String(), err
		}
	} else { //If no specfilter is passed then root filter is applied and iterator filter is used in output filter
		itr := cfg.Filter.ItrSpecFilter[0] + "=='" + crd + "')]"
		for _, f := range cfg.Filter.ItrSpecFilter[1:] {
			itr += f
		}
		getAPIvCmdArgs := []string{"--location", fp, "-t", inputFormat, "--filter"}
		getAPIvCmdArgs = append(getAPIvCmdArgs, cfg.Filter.RootFilter...)
		getAPIvCmdArgs = append(getAPIvCmdArgs, "--o-filter", itr)
		if len(cfg.Filter.ResolveFilter) != 0 {
			getAPIvCmdArgs = append(getAPIvCmdArgs, cfg.Filter.ResolveFilter...)
		}
		schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
		schemaCmd.Stdout = &out
		schemaCmd.Stderr = &er
		err := schemaCmd.Run()
		if err != nil {
			return er.String(), err
		}
	}

	schema := []map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &schema); err != nil {
		return "", err
	}
	if len(schema) == 0 {
		return "", nil
	}
	(schema)[0]["title"] = formatToReadableString(crdname)
	var output []byte
	output, err := json.MarshalIndent(schema[0], "", " ")
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

func getApiVersion(binPath string, fp string, crd string, inputFormat string, cfg Config) (string, error) {
	//few checks to avoid index out of bound panic
	if len(cfg.Filter.ItrFilter) == 0 {
		return "", ErrAbsentFilter(errors.New("Empty ItrFilter"))
	}

	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	itr := cfg.Filter.ItrFilter[0] + "=='" + crd + "')]"
	for _, f := range cfg.Filter.ItrFilter[1:] {
		itr += f
	}
	getAPIvCmdArgs := []string{"--location", fp, "-t", inputFormat, "--filter", itr, "--o-filter"}
	getAPIvCmdArgs = append(getAPIvCmdArgs, cfg.Filter.VersionFilter...)

	schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er
	err := schemaCmd.Run()
	if err != nil {
		return er.String(), err
	}
	grp := []map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &grp); err != nil {
		return "", err
	}
	if len(grp) == 0 {
		return "", err
	}
	var output []byte
	output, err = json.Marshal(grp[0][cfg.Filter.VField])
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(output), "\"", "")
	return s, nil
}
func getApiGrp(binPath string, fp string, crd string, inputFormat string, cfg Config) (string, error) {
	//few checks to avoid index out of bound panic
	if len(cfg.Filter.ItrFilter) == 0 {
		return "", ErrAbsentFilter(errors.New("Empty ItrFilter"))
	}
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	itr := cfg.Filter.ItrFilter[0] + "=='" + crd + "')]"
	for _, f := range cfg.Filter.ItrFilter[1:] {
		itr += f
	}
	getAPIvCmdArgs := []string{"--location", fp, "-t", inputFormat, "--filter", itr, "--o-filter"}
	getAPIvCmdArgs = append(getAPIvCmdArgs, cfg.Filter.GroupFilter...)
	schemaCmd := exec.Command(binPath, getAPIvCmdArgs...)
	schemaCmd.Stdout = &out
	schemaCmd.Stderr = &er

	err := schemaCmd.Run()
	if err != nil {
		return er.String(), err
	}
	grp := []map[string]interface{}{}
	if err := json.Unmarshal(out.Bytes(), &grp); err != nil {
		return "", err
	}
	if len(grp) == 0 {
		return "", err
	}
	var output []byte
	output, err = json.Marshal(grp[0][cfg.Filter.GField])
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(output), "\"", "")
	return s, nil
}

func filterYaml(yamlPath string, filter []string, binPath string, inputFormat string) error {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	filter = append(filter, "-o", "yaml")
	getCrdsCmdArgs := append([]string{"--location", yamlPath, "-t", inputFormat, "--filter"}, filter...)
	cmd := exec.Command(binPath, getCrdsCmdArgs...)
	//emptying buffers
	out.Reset()
	er.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	if err != nil {
		return err
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

//While going from Capital letter to small, insert a whitespace before the capital letter.
//While going from small letter to capital, insert a whitespace after the small letter
func formatToReadableString(input string) string {
	if len(input) == 0 {
		return ""
	}
	finalWord := ""
	for i := range input {
		if i == len(input)-1 {
			break
		}
		switch switchedCasing(input[i], input[i+1]) {
		case 0:
			finalWord += string(input[i])
		case 1:
			finalWord += string(input[i]) + " "
		case -1:
			finalWord += " " + string(input[i])
		}
	}
	return strings.Join(strings.Fields(strings.TrimSpace(finalWord+input[len(input)-1:])), " ")
}

func switchedCasing(a byte, b byte) int {
	aisSmall := int(a) >= 97 && int(a) <= 122
	bisSmall := int(b) >= 97 && int(b) <= 122
	if aisSmall && !bisSmall {
		return 1
	}
	if bisSmall && !aisSmall {
		return -1
	}
	return 0
}
