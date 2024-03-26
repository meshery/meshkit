package generators

import (
	"fmt"

	"github.com/layer5io/meshkit/generators/artifacthub"
	"github.com/layer5io/meshkit/generators/github"
	"github.com/layer5io/meshkit/generators/models"
	"github.com/layer5io/meshkit/utils"
)

const (
	artifactHub = "artifacthub"
	gitHub      = "github"
)

func NewGenerator(registrant, url, packageName string) (models.PackageManager, error) {
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
		}, nil
	}
	return nil, ErrUnsupportedRegistrant(fmt.Errorf("generator not implemented for the registrant %s", registrant))
}
