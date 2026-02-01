package generators

import (
	"fmt"

	"github.com/meshery/meshkit/generators/artifacthub"
	"github.com/meshery/meshkit/generators/github"
	"github.com/meshery/meshkit/generators/models"
	"github.com/meshery/meshkit/utils"
)

const (
	artifactHub = "artifacthub"
	gitHub      = "github"
)

type GeneratorOptions struct {
	Recursive bool
	MaxDepth  int
}

func NewGenerator(registrant, url, packageName string) (models.PackageManager, error) {
	return NewGeneratorWithOptions(registrant, url, packageName, GeneratorOptions{})
}

func NewGeneratorWithOptions(registrant, url, packageName string, opts GeneratorOptions) (models.PackageManager, error) {
	registrant = utils.ReplaceSpacesAndConvertToLowercase(registrant)
	switch registrant {
	case artifactHub:
		return artifacthub.ArtifactHubPackageManager{
			PackageName: packageName,
			SourceURL:   url,
		}, nil
	case gitHub:
		return github.GitHubPackageManager{
			PackageName: packageName,
			SourceURL:   url,
			Recursive:   opts.Recursive,
			MaxDepth:    opts.MaxDepth,
		}, nil
	}
	return nil, ErrUnsupportedRegistrant(fmt.Errorf("generator not implemented for the registrant %s", registrant))
}
