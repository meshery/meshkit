package manifests

const template = "{\"apiVersion\":\"core.oam.dev/v1alpha1\",\"kind\":\"WorkloadDefinition\",\"metadata\":{\"name\":\"\"},\"spec\":{\"definitionRef\":{\"name\":\"\"}}}"
const (
	SERVICE_MESH = iota
	K8s
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
	NamFilter     JsonPath // This will be the json path passed in order to get the names of crds
	NamePath      string
	GroupFilter   JsonPath //This will specify the path to get to group name
	VersionFilter JsonPath //This will specify the path to get to version name. [Version should have a name field]
	SpecFilter    JsonPath //This will specify the path to get spec
}
