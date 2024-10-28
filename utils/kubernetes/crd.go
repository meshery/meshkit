package kubernetes

import (
	"context"

	"github.com/layer5io/meshkit/encoding"
	"github.com/layer5io/meshkit/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type CRD struct {
	Items []CRDItem `json:"items"`
}

type CRDItem struct {
	Spec Spec `json:"spec"`
}

type Spec struct {
	Names    names  `json:"names"`
	Group    string `json:"group"`
	Versions []struct {
		Name string `json:"name"`
	} `json:"versions"`
}

type names struct {
	ResourceName string `json:"plural"`
}

func GetAllCustomResourcesInCluster(ctx context.Context, client rest.Interface) ([]*schema.GroupVersionResource, error) {
	crdresult, err := client.Get().RequestURI("/apis/apiextensions.k8s.io/v1/customresourcedefinitions").Do(context.Background()).Raw()
	if err != nil {
		return nil, err
	}
	var xcrd CRD
	gvks := []*schema.GroupVersionResource{}
	err = encoding.Unmarshal(crdresult, &xcrd)
	if err != nil {
		return nil, err
	}
	for _, c := range xcrd.Items {
		gvks = append(gvks, GetGVRForCustomResources(&c))
	}
	return gvks, nil
}

func GetGVRForCustomResources(crd *CRDItem) *schema.GroupVersionResource {
	return &schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  crd.Spec.Versions[0].Name,
		Resource: crd.Spec.Names.ResourceName,
	}
}

func IsCRD(manifest string) bool {
	cueValue, err := utils.YamlToCue(manifest)
	if err != nil {
		return false
	}
	kind, err := utils.Lookup(cueValue, "kind")
	if err != nil {
		return false
	}
	kindStr, err := kind.String()
	if err != nil {
		return false
	}

	return kindStr == "CustomResourceDefinition"
}
