package main

import (
	"fmt"

	"github.com/layer5io/meshkit/utils/manifests"
)

func main() {
	// rules := clientcmd.NewDefaultClientConfigLoadingRules()
	// configOverrides := &clientcmd.ConfigOverrides{}
	// kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, configOverrides)
	// kcli, err := kubernetes.New([]byte(kubeConfig.ConfigAccess().GetExplicitFile()))
	// if err != nil {
	// 	fmt.Println("bruhhh" + err.Error())

	// 	fmt.Println("bruhhh0")
	// 	return
	// }
	// s, err := kubernetes.GetManifestsFromHelm(kcli, "https://helm.traefik.io/mesh/traefik-mesh-3.0.6.tgz")
	// if err != nil {
	// 	return
	// }

	// fmt.Println(" Ashish " + s)
	v := "v1.9.7"
	m := manifests.Config{Name: "Istio", MeshVersion: v}
	url := "https://raw.githubusercontent.com/istio/istio/1.9.7/manifests/charts/base/crds/crd-all.gen.yaml"
	comp, err := manifests.GetFromManifest(url, manifests.SERVICE_MESH, m)
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
	fmt.Println("Printing stuff: ", comp.Schemas[1])
}
