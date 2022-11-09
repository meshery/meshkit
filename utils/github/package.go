package github

// This package is used when helm chart is fetched directly from github releases
// the format https://github.com/<Owner>/<Repository>/releases/download/Version/<Version/Filename.tgz>

type GithubReleasePackage struct {
	Owner      string
	Repository string
	URL        string
	Version    string
	FileName   string //If this is not passed, the default filename is assumed to be <Version>.tgz (which is usually the case)
}

// Implementing Package Interface which generates component
// func (pkg *GithubReleasePackage) GenerateComponents() ([]v1alpha1.Component, error) {
// 	components := make([]v1alpha1.Component, 0)

// 	crds, err := manifests.GetCrdsFromHelm(pkg.URL)
// 	if err != nil {
// 		return components, ErrComponentGenerate(err)
// 	}
// 	for _, crd := range crds {
// 		comp, err := component.Generate(crd)
// 		if err != nil {
// 			continue
// 		}
// 		comp.Metadata["version"] = pkg.Version
// 		components = append(components, comp)
// 	}
// 	return components, nil
// }

// Implementing Package manager interface which creates the package
// func (pkg *GithubReleasePackage) GetPackage() (models.Package, error) {
// 	if pkg.Owner == "" {
// 		return nil, ErrGetGHPackage(fmt.Errorf("pass a valid github owner for fetching github release helm package"))
// 	}
// 	if pkg.Repository == "" {
// 		return nil, ErrGetGHPackage(fmt.Errorf("pass a valid github repository for fetching github release helm package"))
// 	}
// 	if pkg.Version == "" { //fetch for the latest version
// 		versions, err := utils.GetLatestReleaseTagsSorted(pkg.Owner, pkg.Repository)
// 		if err != nil {
// 			return nil, ErrGetGHPackage(err)
// 		}
// 		if len(versions) == 0 {
// 			return nil, fmt.Errorf("no versions found for given github package")
// 		}
// 		pkg.Version = versions[len(versions)-1]
// 	}
// 	if pkg.FileName == "" {
// 		pkg.FileName = fmt.Sprintf("%s.tgz", pkg.Version)
// 	}
// 	pkg.URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", pkg.Owner, pkg.Repository, pkg.Version, pkg.FileName)
// 	return pkg, nil
// }
