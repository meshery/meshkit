package manifests

import (
	"context"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/yaml"
)

func GenerateComponents(ctx context.Context, manifest string, resource int, cfg Config) (*Component, error) {
	var crds []string
	var c = &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}
	cueCtx := cuecontext.New()
	crds = cfg.ExtractCrds(manifest)
	for _, crd := range crds {
		var parsedCrd cue.Value
		if !cfg.CrdFilter.IsJson {
			file, err := yaml.Extract("crds", crd) // first argument is dummy
			if err != nil {
				// inability to generate component for a single crd should not affect the rest
				// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
				continue
			}
			parsedCrd = cueCtx.BuildFile(file)
		} else {
			expr, err := json.Extract("", []byte(crd))
			if err != nil {
				// inability to generate component for a single crd should not affect the rest
				// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
				continue
			}
			parsedCrd = cueCtx.BuildExpr(expr)
		}
		outDef, err := getDefinitions(parsedCrd, resource, cfg)
		if err != nil {
			// inability to generate component for a single crd should not affect the rest
			// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
			continue
		}
		outSchema, err := getSchema(parsedCrd, cfg)
		if err != nil {
			// inability to generate component for a single crd should not affect the rest
			// TODO: Maintain a list of errors and keep pushing the errors to the list so that it can be displayed at last
			continue
		}
		if cfg.ModifyDefSchema != nil {
			cfg.ModifyDefSchema(&outDef, &outSchema) //definition and schema can be modified using some call back function
		}
		c.Definitions = append(c.Definitions, outDef)
		c.Schemas = append(c.Schemas, outSchema)
	}
	return c, nil
}
