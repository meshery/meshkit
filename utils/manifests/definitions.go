package manifests

const template = "{\"apiVersion\":\"core.oam.dev/v1alpha1\",\"kind\":\"WorkloadDefinition\",\"metadata\":{\"name\":\"\"},\"spec\":{\"definitionRef\":{\"name\":\"\"}}}"
const (
	SERVICE_MESH = iota
	K8
	MESHERY
)

type Component struct {
	Schemas     []string
	Definitions []string
}

// type Components struct {
// 	Versions []Component
// }
type Config struct {
	Name        string // Name of the service mesh,or k8 or meshery
	MeshVersion string
	ApiVersion  string
	Kind        string
}
