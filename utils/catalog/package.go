package catalog

import (
	"fmt"

	"github.com/layer5io/meshkit/models/catalog/v1alpha1"
)

func BuildArtifactHubPkg(name, downloadURL, user, version, createdAt string, catalogData *v1alpha1.CatalogData) *ArtifactHubMetadata {
	artifacthubPkg := &ArtifactHubMetadata{
		Name:        toKebabCase(name),
		DisplayName: name,
		Description: catalogData.PatternInfo,
		Provider: Provider{
			Name: user,
		},
		Links: []Link{
			{
				Name: "download",
				URL:  downloadURL, // this depends on where the design is stored by the user, we can give remote provider URL otherwise
			},
			{
				Name: "Meshery Catalog",
				URL:  "https://meshery.io/catalog",
			},
		},
		HomeURL:   "https://docs.meshery.io/concepts/logical/designs",
		Version:   version,
		CreatedAt: createdAt,
		License:   "Apache-2.0",
		LogoURL:   "https://raw.githubusercontent.com/meshery/meshery.io/0b8585231c6e2b3251d38f749259360491c9ee6b/assets/images/brand/meshery-logo.svg",
		Install:   "mesheryctl design import -f",
		Readme:    fmt.Sprintf("%s \n ##h4 Caveats and Consideration \n", catalogData.PatternCaveats),
	}

	if len(catalogData.SnapshotURL) > 0 {
		artifacthubPkg.Screenshots = append(artifacthubPkg.Screenshots, Screenshot{
			Title: "MeshMap Snapshot",
			URL:   catalogData.SnapshotURL[0],
		})

		if len(catalogData.SnapshotURL) > 1 {
			artifacthubPkg.Screenshots = append(artifacthubPkg.Screenshots, Screenshot{
				Title: "MeshMap Snapshot",
				URL:   catalogData.SnapshotURL[1],
			})
		}
	}

	artifacthubPkg.Screenshots = append(artifacthubPkg.Screenshots, Screenshot{
		Title: "Meshery Project",
		URL:   "https://raw.githubusercontent.com/meshery/meshery.io/master/assets/images/logos/meshery-gradient.png",
	})

	return artifacthubPkg
}

func toKebabCase(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ", "-")

	return s
}
