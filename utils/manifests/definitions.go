package manifests

const workloadDefinitionTemplate = "{\"apiVersion\":\"core.oam.dev/v1alpha1\",\"kind\":\"WorkloadDefinition\",\"metadata\":{\"name\":\"\"},\"spec\":{\"definitionRef\":{\"name\":\"\"}}}"

// Type of resource
const (
	// service mesh resource
	SERVICE_MESH = iota
	// native Kubernetes resource
	K8s
	// native Meshery resource
	MESHERY
)

// Json Paths
type JsonPath []string
type Component struct {
	Schemas     []string
	Definitions []string
}

type Config struct {
	Name        string // Name of the service mesh,or k8 or meshery
	MeshVersion string
	Filter      CrdFilter //json path filters
}

type CrdFilter struct {
	RootFilter    JsonPath //This would be the first filter to get a modified yaml
	NameFilter    JsonPath // This will be the json path passed in order to get the names of crds
	NamePath      string
	GroupFilter   JsonPath //This will specify the path to get to group name
	VersionFilter JsonPath //This will specify the path to get to version name. [Version should have a name field]
	SpecFilter    JsonPath //This will specify the path to get spec
}
