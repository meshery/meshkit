package github

import (
	"fmt"
	"io"
	"os"

	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/component"
	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1/category"
	_component "github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
	"gopkg.in/yaml.v3"
)

type GitHubPackage struct {
	Name       string `yaml:"name" json:"name"`
	filePath   string
	branch     string
	repository string
	version    string
	SourceURL  string `yaml:"source_url" json:"source_url"`
}

func (gp GitHubPackage) GetVersion() string {
	return gp.version
}

func (gp GitHubPackage) GetSourceURL() string {
	return gp.SourceURL
}

func (gp GitHubPackage) GetName() string {
	return gp.Name
}

func (gp GitHubPackage) GenerateComponents(group string) ([]_component.ComponentDefinition, error) {
	components := make([]_component.ComponentDefinition, 0)

	file, err := os.Open(gp.filePath)
	if err != nil {
		return nil, ErrGenerateGitHubPackage(err, gp.Name)
	}

	// parse yaml and break into pages
	decoder := yaml.NewDecoder(file)

	errs := []error{}

	for {
		var crd map[string]any
		if err := decoder.Decode(&crd); err != nil {
			if err == io.EOF {
				break
			}
			// log the error and skip this particular resource
			return components, ErrGenerateGitHubPackage(fmt.Errorf("error decoding yaml: %w , fileName %s", err, gp.filePath), gp.Name)
		}

		resourceBytes, err := yaml.Marshal(crd)
		if err != nil {
			return components, ErrGenerateGitHubPackage(fmt.Errorf("error marshaling yaml: %w", err), gp.Name)
		}

		resource := string(resourceBytes)

		include, err := component.IncludeComponentBasedOnGroup(resource, group)

		if err != nil {
			errs = append(errs, fmt.Errorf("error filtering component by group: %w", err))
		}

		if !include {
			continue
		}

		isCrd := kubernetes.IsCRD(resource)
		if !isCrd {

			comps, err := component.GenerateFromOpenAPI(resource, gp)
			if err != nil {
				errs = append(errs, component.ErrGetSchema(err))
				continue
			}
			components = append(components, comps...)
		} else {

			comp, err := component.Generate(resource)
			if err != nil {
				continue
			}
			if comp.Model == nil {
				comp.Model = &model.ModelDefinition{}
			}
			if comp.Model.Metadata == nil {
				comp.Model.Metadata = &model.ModelDefinition_Metadata{}
			}
			if comp.Model.Metadata.AdditionalProperties == nil {
				comp.Model.Metadata.AdditionalProperties = make(map[string]interface{})
			}

			comp.Model.Metadata.AdditionalProperties["source_uri"] = gp.SourceURL
			comp.Model.Version = gp.version
			comp.Model.Name = gp.Name
			comp.Model.Category = category.CategoryDefinition{
				Name: "",
			}
			comp.Model.DisplayName = manifests.FormatToReadableString(comp.Model.Name)
			components = append(components, comp)
		}

	}

	return components, utils.CombineErrors(errs, "\n")
}
