package artifacthub

import (
	"fmt"
	"strings"

	"github.com/layer5io/meshkit/generators/models"
)

type ArtifactHubPackageManager struct {
	PackageName string
	SourceURL   string
}

func (ahpm ArtifactHubPackageManager) GetPackage() (models.Package, error) {
	// Try to extract package name from URL first
	searchName := ahpm.PackageName
	extractedName := extractPackageNameFromURL(ahpm.SourceURL)

	// If we found a potential package name in the URL, try that first
	if extractedName != "" {
		pkgs, err := GetAhPackagesWithName(extractedName)
		if err == nil && len(pkgs) > 0 {
			// We found packages using the URL-extracted name, use this instead
			searchName = extractedName
		}
	}

	// Get relevant packages using either the extracted name or original name
	pkgs, err := GetAhPackagesWithName(searchName)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		// If still no packages found, try one more time with the original name if we had switched
		if searchName != ahpm.PackageName {
			pkgs, err = GetAhPackagesWithName(ahpm.PackageName)
			if err != nil {
				return nil, err
			}
		}

		if len(pkgs) == 0 {
			return nil, ErrNoPackageFound(searchName, "Artifacthub")
		}
	}

	// update package information
	for i, ap := range pkgs {
		_ = ap.UpdatePackageData()
		pkgs[i] = ap
	}

	// Add filtering/sort based on preferred_models.yaml as well.
	pkgs = SortPackagesWithScore(pkgs)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("could not find any appropriate artifacthub package")
	}

	return pkgs[0], nil
}

// extractPackageNameFromURL tries to get a meaningful package name from an Artifact Hub URL
func extractPackageNameFromURL(url string) string {
	if url == "" {
		return ""
	}

	// Split URL by path segments
	segments := strings.Split(url, "/")
	if len(segments) == 0 {
		return ""
	}

	// The last segment is likely the package name
	packageName := segments[len(segments)-1]

	// Clean up any query parameters
	packageName = strings.Split(packageName, "?")[0]

	// If the URL ends with a slash, use the second-to-last segment
	if packageName == "" && len(segments) > 1 {
		packageName = segments[len(segments)-2]
	}

	return packageName
}
