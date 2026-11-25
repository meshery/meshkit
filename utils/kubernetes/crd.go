package kubernetes

import (
	"context"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/utils"
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

// GetAllCustomResourcesInCluster returns a GroupVersionResource for every CRD in the cluster whose
// group, version and resource name are all set; CRDs that do not yield a complete GVR are skipped.
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
		gvr := GetGVRForCustomResources(&c)
		if gvr.Group == "" || gvr.Version == "" || gvr.Resource == "" {
			continue
		}
		gvks = append(gvks, gvr)
	}
	return gvks, nil
}

// GetGVRForCustomResources returns nil for a nil CRD and an empty version when the CRD has no versions.
func GetGVRForCustomResources(crd *CRDItem) *schema.GroupVersionResource {
	if crd == nil {
		return nil
	}
	var version string
	if len(crd.Spec.Versions) > 0 {
		version = crd.Spec.Versions[0].Name
	}
	return &schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  version,
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
