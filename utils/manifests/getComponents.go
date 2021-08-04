package manifests

import "github.com/layer5io/meshkit/utils"

// func GetIstioManifests() (*Components, error) {
// 	mock := ""
// 	// Getting istio manifests

// 	//
// 	comp, err := generateComponents(mock, SER)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return comp, nil
// }
func GetFromManifest(url string, resource int, cfg Config) (*Component, error) {
	manifest, err := utils.ReadFileSource(url)
	if err != nil {
		return nil, err
	}
	comp, err := generateComponents(manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

// func GetFromHelm(url string, resource int, cfg Config) (*Component, error) {

// }
