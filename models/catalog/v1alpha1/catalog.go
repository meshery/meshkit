package v1alpha1

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/layer5io/meshkit/utils"
	"github.com/meshery/schemas"
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
	Class       ContentClass `json:"class"`
	Description string       `json:"description"`
}

const (
	Official  ContentClass = "official"
	Verified  ContentClass = "verified"
	Community ContentClass = "community"
)

// Load and extract descriptions and enum values from schema
func loadSchemaDescriptions() (map[string]ContentClassObj, error) {
	// Load the schema file
	schema, err := schemas.LoadSchema("schemas/constructs/v1alpha1/catalog_data.json")
	if err != nil {
		return nil, utils.ErrUnmarshal(err)
	}

	properties, propErr := utils.Cast[map[string]interface{}](schema["properties"])
	if propErr != nil {
		return nil, propErr
	}

	classProperty, classErr := utils.Cast[map[string]interface{}](properties["class"])
	if classErr != nil {
		return nil, classErr
	}

	enumDescriptions, _ := utils.Cast[[]interface{}](classProperty["enumDescriptions"])
	enumValues, _ := utils.Cast[[]interface{}](classProperty["enum"])

	descriptions := make(map[string]ContentClassObj)
	for i, value := range enumValues {
		if i < len(enumDescriptions) {
			class := value.(string)
			description := enumDescriptions[i].(string)
			descriptions[class] = ContentClassObj{
				Class:       ContentClass(class),
				Description: description,
			}
		}
	}

	return descriptions, nil
}

// GetCatalogClasses gets class descriptions from the schema
func GetCatalogClasses() []ContentClassObj {
	descriptions, err := loadSchemaDescriptions()
	if err != nil {
		utils.ErrGettingClassDescription(err)
		return nil
	}

	var classObjects []ContentClassObj
	for _, obj := range descriptions {
		classObjects = append(classObjects, obj)
	}

	return classObjects
}

// String method for ContentClass
func (c ContentClass) String() string {
	return string(c)
}
