package manifests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func generateComponents(manifest string, resource int, cfg Config) (*Component, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var binPath string = wd + "/utils/manifests/utilBin/kubeopenapi-jsonschema"
	switch runtime.GOOS {
	case "windows":
		{
			binPath += ".exe"
		}
	case "darwin":
		{
			binPath += "-darwin"
		}
	}

	//make the binary executable
	if err = os.Chmod(binPath, 0750); err != nil {
		return nil, err
	}

	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	path := filepath.Join(os.TempDir(), "/test.yaml")

	c := &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}
	err = populateTempyaml(manifest, path)
	if err != nil {
		return nil, err
	}
	getCrdsCmdArgs := []string{"--location", path, "-t", "yaml", "--filter", "$[?(@.kind==\"CustomResourceDefinition\")]", "-o", "json", "--o-filter", "$..[\"spec\"][\"names\"][\"kind\"]"}
	cmd := exec.Command(binPath, getCrdsCmdArgs...)
	//emptying buffers
	out.Reset()
	er.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &er
	err = cmd.Run()
	if err != nil {
		return nil, ErrGetCrdNames(err)
	}
	crds := getCrdnames(out.String())

	for _, crd := range crds {
		out, err := getDefinitions(template, crd, resource, cfg, path, binPath)
		if err != nil {
			return nil, err
		}
		c.Definitions = append(c.Definitions, out)
		out, err = getSchema(crd, path, binPath)
		if err != nil {
			return nil, ErrGetSchemas(err)
		}
		c.Schemas = append(c.Schemas, out)
	}

	return c, nil
}
