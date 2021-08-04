package main

import (
	"fmt"

	"github.com/layer5io/meshkit/utils/manifests"
)

func main() {
	m := manifests.Config{Name: "Istio", MeshVersion: "v1.9.6"}
	url := "https://raw.githubusercontent.com/istio/istio/1.9.6/manifests/charts/base/crds/crd-all.gen.yaml"
	comp, err := manifests.GetFromManifest(url, manifests.SERVICE_MESH, m)
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
	fmt.Println("Printing stuff: ", comp.Definitions[1])
}
