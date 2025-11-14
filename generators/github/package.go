package github

import (
	"bytes"
	"os"

	"github.com/meshery/meshkit/files"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/component"
	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1/category"
	_component "github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
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

	data, err := os.ReadFile(gp.filePath)
	if err != nil {
		return nil, ErrGenerateGitHubPackage(err, gp.Name)
	}

	manifestBytes := bytes.Split(data, []byte("\n---\n"))
	errs := []error{}

	for _, crd := range manifestBytes {
		// Strip leading comments (like copyright headers) from each YAML document
		cleanedCrd := files.StripLeadingComments(crd)
		resource := string(cleanedCrd)
		include, err := component.IncludeComponentBasedOnGroup(resource, group)

		if err != nil {
			errs = append(errs, err)
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
