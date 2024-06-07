package v1beta1

// CatalogData defines model for catalog_data.
type CatalogData  struct {
// Compatibility A list of technologies included in or implicated by this design; a list of relevant technology tags.
    Compatibility []CatalogDataCompatibility`json:"compatibility"`

// PatternCaveats Specific stipulations to consider and known behaviors to be aware of when using this design.
    PatternCaveats string`json:"pattern_caveats"`

// PatternInfo Purpose of the design along with its intended and unintended uses.
    PatternInfo string`json:"pattern_info"`

// Contains reference to the dark and light mode snapshots of the catalog.
    SnapshotURL []string`json:"imageURL,omitempty"` // this will require updating exisitng catalog data as well. updated the json tag to match previous key name, so changes will not be required in exisitng catgalogs

// Type Categorization of the type of design or operational flow depicted in this design.
    Type CatalogDataType`json:"type"`
}

// CatalogDataCompatibility defines model for CatalogData.Compatibility.
type CatalogDataCompatibility  string

// CatalogDataType Categorization of the type of design or operational flow depicted in this design.
type CatalogDataType  string


func (cd *CatalogData) IsNil() bool {
	return cd == nil || (len(cd.Compatibility) == 0 && 
	cd.PatternCaveats == "" &&
	cd.PatternInfo == "" &&
	cd.Type == "")
}