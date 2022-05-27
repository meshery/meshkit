package manifests

import (
	"cuelang.org/go/cue"
)

// Type of resource
const (
	// service mesh resource
	SERVICE_MESH = iota
	// native Kubernetes resource
	K8s
	// native Meshery resource
	MESHERY
)

type Component struct {
	Schemas     []string
	Definitions []string
}

// all the data that is needed to get a certain value should be present in the config created by the adapter.
type Config struct {
	Name            string                 // Name of the service mesh,or k8 or meshery
	Type            string                 //Type of the workload like- Istio, TraefikMesh, Kuma, OSM, Linkerd,AppMesh,NginxMesh
	MeshVersion     string                 // For service meshes
	K8sVersion      string                 //For K8ss
	ModifyDefSchema func(*string, *string) //takes in definition and schema, does some manipulation on them and returns the new def and schema
	CrdFilter       CueCrdFilter
	ExtractCrds     func(manifest string) []string //takes in the manifest and returns a list of all the crds
}

// takes in the parsed root cue value of the CRD as its input and returns the extracted value
type CueFilter func(rootCRDCueVal cue.Value) (cue.Value, error)

// basically getter functions
// applicable for a single CRD
// can be interpreted as the things that are needed for generating a Component
type CueCrdFilter struct {
	NameExtractor       CueFilter
	GroupExtractor      CueFilter
	VersionExtractor    CueFilter
	SpecExtractor       CueFilter
	IsJson              bool
	IdentifierExtractor CueFilter // identifiers are the values that uniquely identify a CRD (in most of the cases, it is the 'Name' field)
}
