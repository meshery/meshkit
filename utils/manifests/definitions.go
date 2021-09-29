package manifests

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
	Name            string                 // Name of the service mesh,or k8 or meshery
	MeshVersion     string                 // For service meshes
	Filter          CrdFilter              //json path filters
	K8sVersion      string                 //For K8ss
	ModifyDefSchema func(*string, *string) //takes in definition and schema, does some manipulation on them and returns the new def and schema
}

/* How to customize these filters (These comments to be updated if the behavior changes in future)-
There are only two types of filters used internally by kubeopenapi-jsonschema which is being used here- filter(input filter) and output filter.
The filters described below are an abstraction over those filters.
1. RootFilter- is used at two places
	(a) For fetching the crd names or api resources on which we will iterate over, first we apply the root filter to get the objects we are interested in and then
		the NameFilter is applied as output filter to take out just the names from selected objects.
	(b) [this will be discussed after ItrFilter]

2. NameFilter- As explained above, it is used as an --o-filter to extract only the names after RootFilter(1(a)) has been applied.
3. ItrFilter- The first element of the array is an incomplete filter and expected to end with "=='crdname')]". So it needs to be configured just before the double
				equals sign. The rest of the elements of the array are directly appended afterwards to form the complete filter.

4. GroupFilter- After ItrFilter gives us the object with the group and version of the crd/resource we are interested in with this iteration, GroupFilter is used as output filter to only extract the object with group in one of the fields.
5. VersionFilter- After ItrFilter gives us the object with the group and version of the crd/resource we are interested in with this iteration, GroupFilter is used as output filter to only extract the object with version in one of the fields.
6. ItrSpecFilter- Functionally is same as ItrFilter but instead of group and version, it is used to get openapi spec/schema.
7. GField- The GroupFilter returns a json which has group in it. The key name which is used to signify group will be passed here. For eg: "group"
8. VField- The VersionFilter returns a json which has version in it. The key name which is used to signify version will be passed here. For eg: "version", "name","version-name"
9. OnlyRes- In some cases we dont want to compute crdnames/api-resources at runtime as we already have them. Pass those names as an array here to skip that step.
10. IsJson- The file on which to apply all these filters is by default expected to be YAML. Set this to true if a JSON is passed instead. (These are the only two supported formats)
11. SpecFilter- When SpecFilter is passed, it is applied as output filter after ItrSpec filter.
12. ResolveFilter- If passed, this filter will be applied after all the filter for generating schema. This filter is used to resolve $ref in json schema
1(b) If SpecFilter is not passed, then before the ItrSpecFilter the rootfilter will be applied by default and then the ItrSpec filter will be applied as output filter.

*/
type CrdFilter struct {
	RootFilter    JsonPath
	NameFilter    JsonPath
	GroupFilter   JsonPath
	VersionFilter JsonPath
	SpecFilter    JsonPath
	ItrFilter     JsonPath
	ItrSpecFilter JsonPath
	ResolveFilter JsonPath
	VField        string
	GField        string
	IsJson        bool
	OnlyRes       []string
}
