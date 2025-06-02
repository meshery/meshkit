package v1alpha1

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/meshery/meshkit/utils"
)

// CatalogData defines model for catalog_data.
type CatalogData struct {
	ContentClass ContentClass `json:"content_class,omitempty"`
	//Tracks the specific content version that has been made available in the Catalog
	PublishedVersion string `json:"published_version"`

	// Compatibility A list of technologies included in or implicated by this design; a list of relevant technology tags.
	Compatibility []CatalogDataCompatibility `json:"compatibility"`

	// PatternCaveats Specific stipulations to consider and known behaviors to be aware of when using this design.
	PatternCaveats string `json:"pattern_caveats"`

	// PatternInfo Purpose of the design along with its intended and unintended uses.
	PatternInfo string `json:"pattern_info"`

	// Contains reference to the dark and light mode snapshots of the catalog.
	SnapshotURL []string `json:"imageURL,omitempty"` // this will require updating exisitng catalog data as well. updated the json tag to match previous key name, so changes will not be required in exisitng catgalogs

	// Type Categorization of the type of design or operational flow depicted in this design.
	Type CatalogDataType `json:"type"`
}

func (cd *CatalogData) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	data, err := utils.Cast[[]byte](value)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(data), cd)
	if err != nil {
		return utils.ErrUnmarshal(err)
	}
	return nil
}

func (cd CatalogData) Value() (driver.Value, error) {
	marshaledValue, err := json.Marshal(cd)
	if err != nil {
		return nil, utils.ErrMarshal(err)
	}
	return marshaledValue, nil
}

// CatalogDataCompatibility defines model for CatalogData.Compatibility.
type CatalogDataCompatibility string

// CatalogDataType Categorization of the type of design or operational flow depicted in this design.
type CatalogDataType string

func (cd *CatalogData) IsNil() bool {
	return cd == nil || (len(cd.Compatibility) == 0 &&
		cd.PatternCaveats == "" &&
		cd.PatternInfo == "" &&
		cd.Type == "" &&
		cd.ContentClass.String() != "")
}

type ContentClass string

type ContentClassObj struct {
	Class       ContentClass `json:"class"`
	Description string       `json:"description"`
}

const (
	Official  ContentClass = "official"
	Verified  ContentClass = "verified"
	Community ContentClass = "community"
)

func (c ContentClass) String() string {
	switch c {
	case Official:
		return "official"
	case Verified:
		return "verified"
	case Community:
		fallthrough
	default:
		return "community"
	}
}

// Ref to catalog schema - https://github.com/meshery/schemas/blob/master/schemas/constructs/v1alpha1/catalog_data.json
func GetCatalogClasses() []ContentClassObj {
	return []ContentClassObj{
		{
			Class:       Official,
			Description: "Content produced and fully supported by Meshery maintainers. This represents the highest level of support and is considered the most reliable.",
		},
		{
			Class:       Verified,
			Description: "Content produced by partners and verified by Meshery maintainers. While not directly maintained by Meshery, it has undergone a verification process to ensure quality and compatibility.",
		},
		{
			Class:       Community,
			Description: "Content produced and shared by Meshery users. This includes a wide range of content, such as performance profiles, test results, filters, patterns, and applications. Community content may have varying levels of support and reliability.",
		},
	}
}
