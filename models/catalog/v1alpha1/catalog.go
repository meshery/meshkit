package v1alpha1

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/layer5io/meshkit/utils"
)

// CatalogData defines model for catalog_data.
type CatalogData struct {
	ContentClass ContentClass `json:"content_class,omitempty"`
	// Tracks the specific content version that has been made available in the Catalog
	PublishedVersion string `json:"published_version"`

	// Compatibility A list of technologies included in or implicated by this design; a list of relevant technology tags.
	Compatibility []CatalogDataCompatibility `json:"compatibility"`

	// PatternCaveats Specific stipulations to consider and known behaviors to be aware of when using this design.
	PatternCaveats string `json:"pattern_caveats"`

	// PatternInfo Purpose of the design along with its intended and unintended uses.
	PatternInfo string `json:"pattern_info"`

	// Contains reference to the dark and light mode snapshots of the catalog.
	SnapshotURL []string `json:"imageURL,omitempty"` // this will require updating existing catalog data as well. updated the json tag to match previous key name, so changes will not be required in existing catalogs

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
	Class       ContentClass  `json:"class"`
	Description string       `json:"description"`
}

const (
	Official  ContentClass = "official"
	Verified  ContentClass = "verified"
	Community ContentClass = "community"
)

func getClasses() ([]interface{}, error) {
    schema, err := utils.LoadJSONSchema("schemas/constructs/v1alpha1/catalog_data.json")
    if err != nil {
        return nil, utils.ErrUnmarshal(err)
    }
 
    properties, err := utils.Cast[map[string]interface{}](schema["properties"])
	if err != nil {
		return nil, err
	}

    classProperty, err := utils.Cast[map[string]interface{}](properties["class"])
	if err != nil {
		return nil, err
	}

    oneOf, err := utils.Cast[[]interface{}](classProperty["oneOf"])
	if err != nil {
		return nil, err
	}

    return oneOf, nil
}

// GetCatalogClasses gets class and descriptions from the schema
func GetCatalogClasses() []ContentClassObj {
	oneOf, err := getClasses()
	if err != nil {
		utils.ErrGettingClassDescription(err)
		return nil
	}

    var classObjects []ContentClassObj

    for _, item := range oneOf {
        itemMap, _ := utils.Cast[map[string]interface{}](item)
		class, _ := utils.Cast[string](itemMap["const"])
		description, _ := utils.Cast[string](itemMap["description"])
		classObjects = append(classObjects, ContentClassObj{
			Class:       ContentClass(class),
			Description: description,
		})
    }

	return classObjects
}

// String method for ContentClass
func (c ContentClass) String() string {
	return string(c)
}
