package manifests

const template = "{\"apiVersion\":\"core.oam.dev/v1alpha1\",\"kind\":\"WorkloadDefinition\",\"metadata\":{\"name\":\"\"},\"spec\":{\"definitionRef\":{\"name\":\"\"}}}"
const (
	SERVICE_MESH = iota
	K8s
	MESHERY
)

type Component struct {
	Schemas     []string
	Definitions []string
}

type Config struct {
	Name        string // Name of the service mesh,or k8 or meshery
	MeshVersion string
	// Filter      CrdFilter //json path filters
}

// type CrdFilter struct {
// 	CrdFilter     Filter //Filter which will help to get to the specific CRD containing all the info we need.
// 	APIFilter     Filter
// 	VersionFilter Filter
// }
// type Filter struct {
// 	path string
// 	dest string
// }
