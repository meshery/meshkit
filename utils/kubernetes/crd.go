package kubernetes

import (
	"context"

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
	err = utils.Unmarshal(string(crdresult), &xcrd)
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

func IsCRD(manifest map[string]interface{}) bool {
	kind, ok := manifest["kind"].(string)
	return ok && kind == "CustomResourceDefinition"
}
