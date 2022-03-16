package manifests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/layer5io/meshkit/utils"
)

// GenerateComponents generates components given manifest(yaml/json) ,resource type, and additional configuration
func GenerateComponents(ctx context.Context, manifest string, resource int, cfg Config) (*Component, error) {
	var inputFormat = "yaml"
	if cfg.Filter.IsJson {
		inputFormat = "json"
	}
	wd := filepath.Join(utils.GetHome(), ".meshery", "bin")
	err := os.Mkdir(wd, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, ErrCreatingDirectory(err)
	}
	fmt.Println("Looking for kubeopenapi-jsonschema in ", wd)
	var binPath string = filepath.Join(wd, "kubeopenapi-jsonschema")
	var url string = "https://github.com/layer5io/kubeopenapi-jsonschema/releases/download/v0.1.2/kubeopenapi-jsonschema"
	switch runtime.GOOS {
	case "windows":
		binPath += ".exe"
		url += ".exe"
	case "darwin":
		binPath += "-darwin"
		url += "-darwin"

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
	path := filepath.Join(wd, "test.yaml")
	removeHelmTemplatingFromCRD(&manifest)
	err = populateTempyaml(manifest, path)
	if err != nil {
		return nil, err
	}
	var crds []string
	var c = &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}
	if len(cfg.Filter.OnlyRes) == 0 { //If the resources are not given by default, then extract using filter
		var (
			out bytes.Buffer
			er  bytes.Buffer
		)
		filteroot := cfg.Filter.RootFilter
		err = filterYaml(ctx, path, filteroot, binPath, inputFormat)
		if err != nil {
			return nil, err
		}
		filter := cfg.Filter.NameFilter
		filteroot = append(filteroot, "-o", "json", "--o-filter")
		filteroot = append(filteroot, filter...)
		getCrdsCmdArgs := append([]string{"--location", path, "-t", inputFormat, "--filter"}, filteroot...)
		cmd := exec.CommandContext(ctx, binPath, getCrdsCmdArgs...)
		//emptying buffers
		out.Reset()
		er.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &er
		err = cmd.Run()
		if err != nil {
			return nil, ErrGetCrdNames(err)
		}
		crds = getCrdnames(out.String())
	} else {
		crds = cfg.Filter.OnlyRes
	}

	for _, crd := range crds {
		outDef, err := getDefinitions(crd, resource, cfg, path, binPath, ctx)
		if err != nil {
			return nil, err
		}
		outSchema, err := getSchema(crd, path, binPath, cfg, ctx)
		if err != nil {
			return nil, ErrGetSchemas(err)
		}
		if cfg.ModifyDefSchema != nil {
			cfg.ModifyDefSchema(&outDef, &outSchema) //definition and schema can be modified using some call back function
		}
		c.Definitions = append(c.Definitions, outDef)
		c.Schemas = append(c.Schemas, outSchema)
	}
	err = deleteFile(path)
	if err != nil {
		fmt.Println("error in cleanup" + err.Error())
	}
	return c, nil
}
