package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"encoding/json"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/files"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/csv"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1alpha1/capability"
	schmeaVersion "github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas"
)

const (
	SVG_WIDTH  = 20
	SVG_HEIGHT = 20
)

type ComponentCSV struct {
	Registrant string `json:"registrant" csv:"registrant"`
	Model      string `json:"model" csv:"model"`
	Component  string `json:"component" csv:"component"`
	// Category           category.CategoryDefinitionName   `json:"category" csv:"category"`
	// SubCategory        subcategory.SubCategoryDefinition `json:"subCategory" csv:"subCategory"`
	Description        string `json:"description" csv:"description"`
	Shape              string `json:"shape" csv:"shape"`
	PrimaryColor       string `json:"primaryColor" csv:"primaryColor"`
	SecondaryColor     string `json:"secondaryColor" csv:"secondaryColor"`
	SVGColor           string `json:"svgColor" csv:"svgColor"`
	SVGWhite           string `json:"svgWhite" csv:"svgWhite"`
	SVGComplete        string `json:"svgComplete" csv:"svgComplete"`
	Schema             string `json:"schema" csv:"schema"`
	Docs               string `json:"docs" csv:"docs"`
	StyleOverrides     string `json:"styleOverrides" csv:"styleOverrides"`
	Styles             string `json:"styles" csv:"styles"`
	ShapePolygonPoints string `json:"shapePolygonPoints" csv:"shapePolygonPoints"`
	DefaultData        string `json:"defaultData" csv:"defaultData"`
	Capabilities       string `json:"capabilities" csv:"capabilities"`
	LogoURL            string `json:"logoURL" csv:"logoURL"`
	Genealogy          string `json:"genealogy" csv:"genealogy"`
	IsAnnotation       string `json:"isAnnotation" csv:"isAnnotation"`
	Version            string `json:"version" csv:"version"`

	ModelDisplayName string `json:"modelDisplayName" csv:"-"`

	Status string `json:"status" csv:"status"`
}

// The Component Definition generated assumes or is only for components which have registrant as "meshery"
func (c *ComponentCSV) CreateComponentDefinition(isModelPublished bool, defVersion string) (component.ComponentDefinition, error) {
	status := entity.Enabled
	if c.Status != "" {
		if utils.ReplaceSpacesAndConvertToLowercase(c.Status) == "false" {
			status = entity.Ignored
		}
	}
	componentDefinition := &component.ComponentDefinition{
		SchemaVersion: schmeaVersion.ComponentSchemaVersion,
		DisplayName:   c.Component,
		Format:        "JSON",
		Version:       defVersion,
		Metadata: component.ComponentDefinition_Metadata{
			Published: isModelPublished,
		},
		Status: (*component.ComponentDefinitionStatus)(&status),
		Component: component.Component{
			Kind:    c.Component,
			Schema:  c.Schema,
			Version: c.Version,
		},
	}
	if c.Description != "" {
		componentDefinition.Description = c.Description
	}
	err := c.UpdateCompDefinition(componentDefinition)
	return *componentDefinition, err
}

var compMetadataValues = []string{
	"genealogy", "isAnnotation", "styleOverrides",
}
var compStyleValues = []string{
	"primaryColor", "secondaryColor", "svgColor", "svgWhite", "svgComplete", "shape",
}

func (c *ComponentCSV) UpdateCompDefinition(compDef *component.ComponentDefinition) error {
	status := entity.Enabled
	if c.Status != "" {
		if utils.ReplaceSpacesAndConvertToLowercase(c.Status) == "false" {
			status = entity.Ignored
		}
	}
	compDef.Status = (*component.ComponentDefinitionStatus)(&status)
	var existingAddditionalProperties map[string]interface{}
	if c.Description != "" {
		compDef.Description = c.Description
	}
	if c.Schema != "" {
		compDef.Component.Schema = c.Schema
	}
	if c.Version != "" {
		compDef.Component.Version = c.Version
	}
	if compDef.Metadata.AdditionalProperties != nil {
		existingAddditionalProperties = compDef.Metadata.AdditionalProperties
	}

	metadata := map[string]interface{}{}
	compMetadata, err := utils.MarshalAndUnmarshal[ComponentCSV, map[string]interface{}](*c)
	if err != nil {
		return err
	}
	var capabilities []capability.Capability
	if c.Capabilities != "" && c.Capabilities != "null" {
		err := encoding.Unmarshal([]byte(c.Capabilities), &capabilities)
		if err != nil {
			Log.Error(err)
			defaultCapabilities, defaultErr := getMinimalUICapabilitiesFromSchema()
			if defaultErr == nil {
				capabilities = defaultCapabilities
			}
		}
	} else {
		defaultCapabilities, err := getMinimalUICapabilitiesFromSchema()
		if err != nil {
			Log.Error(err)
		} else {
			capabilities = defaultCapabilities
		}
	}

	compDef.Capabilities = &capabilities
	compDefStyles := &component.Styles{}

	//Addtional properties from file
	for key, value := range existingAddditionalProperties {
		metadata[key] = value
	}

	//metadata properties from csv
	for _, key := range compMetadataValues {
		if key == "genealogy" {
			genealogy, err := utils.Cast[string](compMetadata[key])
			if err == nil {
				compDef.Metadata.Genealogy = genealogy
			}
		} else if key == "isAnnotation" {
			if strings.ToLower(c.IsAnnotation) == "true" {
				compDef.Metadata.IsAnnotation = true
			} else {
				compDef.Metadata.IsAnnotation = false
			}
		} else if key == "styleOverrides" {
			if c.StyleOverrides != "" {
				err := encoding.Unmarshal([]byte(c.StyleOverrides), &compDefStyles)
				if err != nil {
					return err
				}
			}
		} else {
			metadata[key] = compMetadata[key]
		}
	}

	//styling properties from csv
	for _, key := range compStyleValues {
		if c.Shape != "" {
			shape := c.Shape
			compDefStyles.Shape = (*component.ComponentDefinitionStylesShape)(&shape)
		}
		if c.PrimaryColor != "" {

			compDefStyles.PrimaryColor = c.PrimaryColor
		}
		if c.SecondaryColor != "" {
			compDefStyles.SecondaryColor = &c.SecondaryColor
		}
		if key == "svgColor" {
			compDefStyles.SvgColor, err = utils.Cast[string](compMetadata[key])
			if err != nil {
				compDefStyles.SvgColor = c.SVGColor
			}
		}
		if key == "svgWhite" {
			compDefStyles.SvgWhite, err = utils.Cast[string](compMetadata[key])
			if err != nil {
				compDefStyles.SvgWhite = c.SVGWhite
			}
		}
		if key == "svgComplete" {
			compDefStyles.SvgComplete, err = utils.Cast[string](compMetadata[key])
			if err != nil {
				compDefStyles.SvgComplete = c.SVGComplete
			}
		}

	}
	compDef.Styles = compDefStyles
	compDef.Metadata.AdditionalProperties = metadata
	return nil
}



type ComponentCSVHelper struct {
	SpreadsheetID  int64
	SpreadsheetURL string
	Title          string
	CSVPath        string
	Components     map[string]map[string][]ComponentCSV
}

func NewComponentCSVHelper(sheetURL, spreadsheetName string, spreadsheetID int64, localCsvPath string) (*ComponentCSVHelper, error) {
	var csvPath string
	if localCsvPath == "" {
		dirPath := filepath.Join(utils.GetHome(), ".meshery", "content")
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return nil, utils.ErrCreateDir(err, dirPath)
		}

		csvPath = filepath.Join(dirPath, "components.csv")

		sheetURL, err = DownloadCSVAndGetDownloadURL(sheetURL, csvPath, spreadsheetID)
		if err != nil {
			return nil, err
		}

	} else {
		csvPath = localCsvPath
	}

	return &ComponentCSVHelper{
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheetURL,
		Title:          spreadsheetName,
		CSVPath:        csvPath,
		Components:     make(map[string]map[string][]ComponentCSV),
	}, nil
}

func (mch *ComponentCSVHelper) GetColumns() ([]string, error) {
	csvReader, err := csv.NewCSVParser[ComponentCSV](mch.CSVPath, rowIndex, nil, func(_ []string, _ []string) bool {
		return true
	})
	if err != nil {
		return nil, err
	}

	return csvReader.ExtractCols(rowIndex)
}

func (mch *ComponentCSVHelper) ParseComponentsSheet(modelName string) error {
	ch := make(chan ComponentCSV, 1)
	errorChan := make(chan error, 1)
	csvReader, err := csv.NewCSVParser[ComponentCSV](mch.CSVPath, rowIndex, nil, func(_ []string, _ []string) bool {
		return true
	})

	if err != nil {
		return files.ErrFileRead(err)
	}

	go func() {
		Log.Info("Parsing Components...")

		err := csvReader.Parse(ch, errorChan)
		if err != nil {
			errorChan <- err
		}
	}()

	for {
		select {

		case data := <-ch:
			if modelName != "" && data.Model != modelName {
				continue
			}
			if mch.Components[data.Registrant] == nil {
				mch.Components[data.Registrant] = make(map[string][]ComponentCSV, 0)
			}
			if mch.Components[data.Registrant][data.Model] == nil {
				mch.Components[data.Registrant][data.Model] = make([]ComponentCSV, 0)
			}
			mch.Components[data.Registrant][data.Model] = append(mch.Components[data.Registrant][data.Model], data)
			// Log.Info(fmt.Sprintf("Reading registrant [%s] model [%s] component [%s]", data.Registrant, data.Model, data.Component))
		case err := <-errorChan:
			Log.Error(err)

		case <-csvReader.Context.Done():
			return nil
		}
	}
}

func CreateComponentsMetadataAndCreateSVGsForMDXStyle(model ModelCSV, components []ComponentCSV, path, svgDir string) (string, error) {
	err := os.MkdirAll(filepath.Join(path, svgDir), 0777)
	if err != nil {
		return "", err
	}
	componentMetadata := `[`
	for idx, comp := range components {
		componentTemplate := `
{
"name": "%s",
"colorIcon": "%s",
"whiteIcon": "%s",
"description": "%s",
}`

		// add comma if not last component
		if idx != len(components)-1 {
			componentTemplate += ","
		}

		compName := utils.FormatName(manifests.FormatToReadableString(comp.Component))
		colorIconDir := filepath.Join(svgDir, compName, "icons", "color")
		whiteIconDir := filepath.Join(svgDir, compName, "icons", "white")

		componentMetadata += fmt.Sprintf(componentTemplate, compName, fmt.Sprintf("%s/%s-color.svg", colorIconDir, compName), fmt.Sprintf("%s/%s-white.svg", whiteIconDir, compName), comp.Description)

		// create color svg dir
		err = os.MkdirAll(filepath.Join(path, colorIconDir), 0777)
		if err != nil {
			return "", err
		}

		// create white svg dir
		err = os.MkdirAll(filepath.Join(path, whiteIconDir), 0777)
		if err != nil {
			return "", err
		}

		colorSVG, whiteSVG := getSVGForComponent(model, comp)
		err = utils.WriteToFile(filepath.Join(path, colorIconDir, compName+"-color.svg"), colorSVG)
		if err != nil {
			return "", err
		}
		err = utils.WriteToFile(filepath.Join(path, whiteIconDir, compName+"-white.svg"), whiteSVG)
		if err != nil {
			return "", err
		}
	}

	componentMetadata += `]`

	return componentMetadata, nil
}

func CreateRelationshipsMetadata(model ModelCSV, relationships []RelationshipCSV) (string, error) {

	relationshipMetadata := ""
	for _, relnship := range relationships {
		relationshipTemplate := `
- type: "%s"
  kind: "%s"
  description: "%s"`

		relnshipType := relnship.Type
		relnshipKind := relnship.KIND
		relnshipDescription := relnship.Description

		relationshipMetadata += fmt.Sprintf(relationshipTemplate, relnshipType, relnshipKind, relnshipDescription)

	}
	return relationshipMetadata, nil

}

func CreateComponentsMetadataAndCreateSVGsForMDStyle(model ModelCSV, components []ComponentCSV, path, svgDir string) (string, error) {
	err := os.MkdirAll(filepath.Join(path), 0777)
	if err != nil {
		return "", err
	}
	componentMetadata := ""
	for _, comp := range components {
		componentTemplate := `
- name: %s
  colorIcon: %s
  whiteIcon: %s
  description: %s`

		compName := utils.FormatName(manifests.FormatToReadableString(comp.Component))
		colorIconDir := filepath.Join(svgDir, compName, "icons", "color")
		whiteIconDir := filepath.Join(svgDir, compName, "icons", "white")

		componentMetadata += fmt.Sprintf(componentTemplate, compName, fmt.Sprintf("%s/%s-color.svg", colorIconDir, compName), fmt.Sprintf("%s/%s-white.svg", whiteIconDir, compName), comp.Description)

		// create color svg dir
		err = os.MkdirAll(filepath.Join(path, compName, "icons", "color"), 0777)
		if err != nil {
			return "", err
		}

		// create white svg dir
		err = os.MkdirAll(filepath.Join(path, compName, "icons", "white"), 0777)
		if err != nil {
			return "", err
		}

		colorSVG, whiteSVG := getSVGForComponent(model, comp)
		err = utils.WriteToFile(filepath.Join(path, compName, "icons", "color", compName+"-color.svg"), colorSVG)
		if err != nil {
			return "", err
		}
		err = utils.WriteToFile(filepath.Join(path, compName, "icons", "white", compName+"-white.svg"), whiteSVG)
		if err != nil {
			return "", err
		}
	}

	return componentMetadata, nil
}

func (m ComponentCSVHelper) Cleanup() error {
	// remove csv file
	Log.Info("Removing CSV file: ", m.CSVPath)
	err := os.Remove(m.CSVPath)
	if err != nil {
		return err
	}
	return nil
}

func ConvertCompDefToCompCSV(modelcsv *ModelCSV, compDef component.ComponentDefinition) *ComponentCSV {
	compCSV, _ := utils.MarshalAndUnmarshal[map[string]interface{}, ComponentCSV](compDef.Metadata.AdditionalProperties)
	compCSV.Registrant = modelcsv.Registrant
	compCSV.Model = modelcsv.Model
	compCSV.Component = compDef.Component.Kind
	compCSV.ModelDisplayName = modelcsv.ModelDisplayName
	compCSV.Capabilities = modelcsv.Capabilities
	compCSV.Shape = modelcsv.Shape
	compCSV.PrimaryColor = modelcsv.PrimaryColor
	compCSV.SecondaryColor = modelcsv.SecondaryColor
	compCSV.SVGColor = modelcsv.SVGColor
	compCSV.SVGWhite = modelcsv.SVGWhite
	compCSV.SVGComplete = modelcsv.SVGComplete
	compCSV.Genealogy = compDef.Metadata.Genealogy
	compCSV.IsAnnotation = strconv.FormatBool(compDef.Metadata.IsAnnotation)
	return &compCSV
}

func getSVGForComponent(model ModelCSV, component ComponentCSV) (colorSVG string, whiteSVG string) {
	colorSVG = component.SVGColor
	whiteSVG = component.SVGWhite

	if colorSVG == "" {
		colorSVG = model.SVGColor
	}

	if whiteSVG == "" {
		whiteSVG = model.SVGWhite
	}
	return
}


func getMinimalUICapabilitiesFromSchema() ([]capability.Capability, error) {
    schema, err := schemas.Schemas.ReadFile("schemas/constructs/v1beta1/component/component.json")
    if err != nil {
        return nil, fmt.Errorf("failed to read component schema: %v", err)
    }

    capabilitiesJSON, err := extractCapabilitiesJSONFromSchema(schema)
    if err != nil {
        return nil, fmt.Errorf("failed to extract capabilities from schema: %v", err)
    }

    var allCapabilities []capability.Capability
    if err := json.Unmarshal(capabilitiesJSON, &allCapabilities); err != nil {
        return nil, fmt.Errorf("failed to unmarshal capabilities: %v", err)
    }

    if len(allCapabilities) >= 3 {
        return allCapabilities[len(allCapabilities)-3:], nil
    }

    return nil, fmt.Errorf("insufficient default capabilities in schema, found %d", len(allCapabilities))
}

func extractCapabilitiesJSONFromSchema(schema []byte) ([]byte, error) {
    var schemaMap map[string]interface{}
    if err := json.Unmarshal(schema, &schemaMap); err != nil {
        return nil, err
    }

    if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
        if capabilitiesSchema, ok := properties["capabilities"].(map[string]interface{}); ok {
            if defaultValue, ok := capabilitiesSchema["default"]; ok {
                return json.Marshal(defaultValue)
            }
        }
    }

    return nil, fmt.Errorf("default capabilities not found in schema")
}