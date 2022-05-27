package manifests

import (
	"context"
	"fmt"

	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/yaml"
)

func GenerateComponents(ctx context.Context, manifest string, resource int, cfg Config) (*Component, error) {
	var crds []string
	var c = &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}
	crds = cfg.ExtractCrds(manifest)
	for _, crd := range crds {
		file, err := yaml.Extract("crds", crd) // first arguement is dummy
		if err != nil {
			return nil, err
		}
		cueCtx := cuecontext.New()
		parsedCrd := cueCtx.BuildFile(file)
		outDef, err := getDefinitions(parsedCrd, resource, cfg, ctx)
		if err != nil {
			// inability to generate component for a single crd should not affect the rest
			// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
			continue
			// return nil, err
		}
		outSchema, err := getSchema(parsedCrd, cfg, ctx)
		if err != nil {
			// inability to generate component for a single crd should not affect the rest
			// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
			continue
			// return nil, ErrGetSchemas(err)
		}
		if cfg.ModifyDefSchema != nil {
			cfg.ModifyDefSchema(&outDef, &outSchema) //definition and schema can be modified using some call back function
		}
		c.Definitions = append(c.Definitions, outDef)
		c.Schemas = append(c.Schemas, outSchema)
	}
	fmt.Printf("%v", c)
	return c, nil
}
