package manifests

import (
	"context"
	"cuelang.org/go/cue/cuecontext"
)

func GenerateComponents(ctx context.Context, manifest string, resource int, cfg Config) (*Component, error) {

	var crds []string
	var c = &Component{
		Schemas:     []string{},
		Definitions: []string{},
	}

	cueCtx := cuecontext.New()
	parsedManifest := cueCtx.CompileString(manifest) // parsing the manifest into cue Value

	if len(cfg.Filter.OnlyRes) == 0 { //If the resources are not given by default, then extract using filter
		var err error
		crds, err = cfg.CueFilter.GetResourceIdentifiersList(parsedManifest)
		if err != nil {
			return nil, err
		}
	} else {
		crds = cfg.Filter.OnlyRes
	}

	for _, crd := range crds {
		outDef, err := getDefinitions(crd, resource, parsedManifest, cfg, ctx)
		if err != nil {
			return nil, err
		}
		outSchema, err := getSchema(crd, parsedManifest, cfg, ctx)
		if err != nil {
			return nil, ErrGetSchemas(err)
		}
		if cfg.ModifyDefSchema != nil {
			cfg.ModifyDefSchema(&outDef, &outSchema) //definition and schema can be modified using some call back function
		}
		c.Definitions = append(c.Definitions, outDef)
		c.Schemas = append(c.Schemas, outSchema)
	}

	return c, nil
}
