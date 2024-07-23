package registry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"path/filepath"
	"google.golang.org/api/sheets/v4"
	"github.com/layer5io/meshkit/utils/store"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/csv"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
)

var (
	rowIndex = 1
	shouldRegisterColIndex = -1
	shouldRegisterMod = "publishToSites"
	modelToCompGenerateTracker = store.NewGenericThreadSafeStore[CompGenerateTracker]()
)

var modelMetadataValues = []string{
	"primaryColor", "secondaryColor", "svgColor", "svgWhite", "svgComplete", "styleOverrides", "styles", "shapePolygonPoints", "defaultData", "capabilities", "isAnnotation", "shape",
}

func NewModelCSVHelper(sheetURL, spreadsheetName string, spreadsheetID int64) (*ModelCSVHelper, error) {
	sheetURL = sheetURL + "/pub?output=csv" + "&gid=" + strconv.FormatInt(spreadsheetID, 10)
	Log.Info("Downloading CSV from: ", sheetURL)
	dirPath := filepath.Join(utils.GetHome(), ".meshery", "content")
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return nil, ErrCreateDir(err, dirPath)
	}
	csvPath := filepath.Join(dirPath, "models.csv")
	err = utils.DownloadFile(csvPath, sheetURL)
	if err != nil {
		return nil, utils.ErrReadingRemoteFile(err)
	}

	return &ModelCSVHelper{
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheetURL,
		Models:         []ModelCSV{},
		CSVPath:        csvPath,
		Title:          spreadsheetName,
	}, nil
}

func GetSheetIDFromTitle(s *sheets.Spreadsheet, title string) int64 {
	for _, sheet := range s.Sheets {
		if sheet.Properties.Title == title {
			return sheet.Properties.SheetId
		}
	}
	return -1
}

func (mch *ModelCSVHelper) ParseModelsSheet(parseForDocs bool) error {
	ch := make(chan ModelCSV, 1)
	errorChan := make(chan error, 1)
	csvReader, err := csv.NewCSVParser[ModelCSV](mch.CSVPath, rowIndex, nil, func(columns []string, currentRow []string) bool {
		index := 0

		if parseForDocs {
			index = GetIndexForRegisterCol(columns, shouldRegisterMod)
		} else {
			// Generation of models should not consider publishedToRegistry column value.
			// Generation should happen for all models, while during registration "published" attribute should be respected.
			return true
		}
		if index != -1 && index < len(currentRow) {
			shouldRegister := currentRow[index]
			return strings.ToLower(shouldRegister) == "true"
		}
		return false
	})

	if err != nil {
		return ErrFileRead(err)
	}

	go func() {
		Log.Info("Parsing Models...")
		err := csvReader.Parse(ch, errorChan)
		if err != nil {
			errorChan <- err
		}
	}()
	for {
		select {

		case data := <-ch:
			mch.Models = append(mch.Models, data)
			Log.Info(fmt.Sprintf("Reading registrant [%s] model [%s]", data.Registrant, data.Model))
		case err := <-errorChan:
			return ErrFileRead(err)

		case <-csvReader.Context.Done():
			return nil
		}
	}
}

func (mcv *ModelCSV) CreateModelDefinition(version, defVersion string) v1beta1.Model {
	status := entity.Ignored
	if strings.ToLower(mcv.PublishToRegistry) == "true" {
		status = entity.Enabled
	}

	model := v1beta1.Model{
		VersionMeta: v1beta1.VersionMeta{
			Version:       defVersion,
			SchemaVersion: v1beta1.ModelSchemaVersion,
		},
		Name:        mcv.Model,
		DisplayName: mcv.ModelDisplayName,
		Status:      status,
		Registrant: v1beta1.Host{
			Hostname: utils.ReplaceSpacesAndConvertToLowercase(mcv.Registrant),
		},
		Category: v1beta1.Category{
			Name: mcv.Category,
		},
		SubCategory: mcv.SubCategory,
		Model: v1beta1.ModelEntity{
			Version: version,
		},
	}
	err := mcv.UpdateModelDefinition(&model)
	if err != nil {
		Log.Error(err)
	}
	return model
}

func (m *ModelCSV) UpdateModelDefinition(modelDef *v1beta1.Model) error {

	metadata := map[string]interface{}{}
	modelMetadata, err := utils.MarshalAndUnmarshal[ModelCSV, map[string]interface{}](*m)
	if err != nil {
		return err
	}
	metadata = utils.MergeMaps(metadata, modelDef.Metadata)

	for _, key := range modelMetadataValues {
		if key == "svgColor" || key == "svgWhite" {
			svg, err := utils.Cast[string](modelMetadata[key])
			if err == nil {
				metadata[key], err = utils.UpdateSVGString(svg, SVG_WIDTH, SVG_HEIGHT)
				if err != nil {
					// If svg cannot be updated, assign the svg value as it is
					metadata[key] = modelMetadata[key]
				}
			}
		}
		metadata[key] = modelMetadata[key]
	}

	isAnnotation := false
	if strings.ToLower(m.IsAnnotation) == "true" {
		isAnnotation = true
	}
	metadata["isAnnotation"] = isAnnotation
	modelDef.Metadata = metadata
	return nil
}

func GetIndexForRegisterCol(cols []string, shouldRegister string) int {
	if shouldRegisterColIndex != -1 {
		return shouldRegisterColIndex
	}

	for index, col := range cols {
		if col == shouldRegister {
			return index
		}
	}
	return shouldRegisterColIndex
}