package manifests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/layer5io/meshkit/utils"
)

func generateComponents(manifest string, resource int, cfg Config) (*Component, error) {
	wd := utils.GetHome() + "/.meshery/bin"
	fmt.Println("Looking for kubeopenapi-jsonschema in ", wd)
	var binPath string = wd + "/kubeopenapi-jsonschema"
	var url string = "https://github.com/layer5io/kubeopenapi-jsonschema/releases/download/v0.1.0/kubeopenapi-jsonschema"
	switch runtime.GOOS {
	case "windows":
		{
			binPath += ".exe"
			url += ".exe"
		}
	case "darwin":
		{
			binPath += "-darwin"
			url += "-darwin"
		}
	}
	//download the binary on that path if it doesn't exist
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		fmt.Println("Downloading kubeopenapi-jsonschema at " + binPath + "...")
		errdownload := utils.DownloadFile(binPath, url)
		if errdownload != nil {
			return nil, errdownload
		}
		fmt.Println("Download Completed")
	}

	//make the binary executable
	if err := os.Chmod(binPath, 0750); err != nil {
		return nil, err
	}

	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	path := filepath.Join(wd, "/test.yaml")
	err := populateTempyaml(manifest, path)
	if err != nil {
		return nil, err
	}
	filteroot := cfg.Filter.RootFilter //cfg.Filter.RootFilter
	err = filterYaml(path, filteroot, binPath)
	if err != nil {
		return nil, err
	}
	c := &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}
	filter := cfg.Filter.NameFilter //cfg.Filter.Name
	filteroot = append(filteroot, "-o", "json", "--o-filter")
	filteroot = append(filteroot, filter...)
	getCrdsCmdArgs := append([]string{"--location", path, "-t", "yaml", "--filter"}, filteroot...)
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
		out, err := getDefinitions(workloadDefinitionTemplate, crd, resource, cfg, path, binPath)
		if err != nil {
			return nil, err
		}
		c.Definitions = append(c.Definitions, out)
		out, err = getSchema(crd, path, binPath, cfg)
		if err != nil {
			return nil, ErrGetSchemas(err)
		}
		c.Schemas = append(c.Schemas, out)
	}
	err = deleteFile(path)
	if err != nil {
		fmt.Println("error in cleanup" + err.Error())
	}
	return c, nil
}
