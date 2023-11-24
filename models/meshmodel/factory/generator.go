package factory


import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"time"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils/artifacthub"
	"github.com/layer5io/meshkit/utils"
	"gopkg.in/yaml.v3"
)

// TODO: make configurable; take as param
var (
	AhSearchEndpoint = artifacthub.AhHelmExporterEndpoint

	OutputDirectoryPath     = "../../server/meshmodel"
	ComponentModelsFileName = path.Join(OutputDirectoryPath, "component_models.yaml")
)

func GenerateModels(spreadsheetID string, sheetID int) {
	if _, err := os.Stat(OutputDirectoryPath); err != nil {
		err := os.Mkdir(OutputDirectoryPath, 0744)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	modelsFd, err := os.OpenFile(ComponentModelsFileName, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	modelsWriter := Writer{
		file: modelsFd,
	}
	defer modelsFd.Close()
	// move to a new function: getHelmPackages
	pkgs := make([]artifacthub.AhPackage, 0)
	content, err := io.ReadAll(modelsFd)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = yaml.Unmarshal(content, &pkgs)
	if err != nil {
		fmt.Println(err)
	}

	if len(pkgs) == 0 {
		pkgs, err = artifacthub.GetAllAhHelmPackages()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = WriteComponentModels(pkgs, &modelsWriter)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	verified, official, cncf, priority, unverified := artifacthub.SortOnVerified(pkgs)
	csvChan := make(chan string, 50)
	f, err := os.Create(dumpFile)
	if err != nil {
		fmt.Printf("Error creating file: %s\n", err)
	}

	f.Write([]byte("model,component_count,components\n"))
	go func() {
		for entry := range csvChan {
			f.Write([]byte(entry))
		}
	}()
	srv := utils.NewSheetSRV()
	// Convert sheet ID to sheet name.
	response1, err := srv.Spreadsheets.Get(spreadsheetID).Fields("sheets(properties(sheetId,title))").Do()
	if err != nil || response1.HTTPStatusCode != 200 {
		fmt.Println(err)
		return
	}
	sheetName := ""
	for _, v := range response1.Sheets {
		prop := v.Properties
		if prop.SheetId == int64(sheetID) {
			sheetName = prop.Title
			break
		}
	}
	// Set the range of cells to retrieve.
	rangeString := sheetName + COLUMNRANGE

	// Get the value of the specified cell.
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, rangeString).Do()
	if err != nil {
		fmt.Println("Unable to retrieve data from sheet: ", err)
		return
	}
	availableModels := make(map[string][]interface{})
	availableComponentsPerModel := make(map[string]map[string]bool)
	for _, val := range resp.Values {
		if len(val) > utils.NameToIndex["model"]+1 {
			key := val[utils.NameToIndex["model"]].(string)
			if key == "" {
				continue
			}
			var compkey string
			if len(val) > utils.NameToIndex["component"]+1 {
				compkey = val[utils.NameToIndex["component"]].(string)
			}
			if compkey == "" {
				availableModels[key] = make([]interface{}, len(val))
				copy(availableModels[key], val)
				continue
			}
			if availableComponentsPerModel[key] == nil {
				availableComponentsPerModel[key] = make(map[string]bool)
			}
			availableComponentsPerModel[key][compkey] = true
		}
	}

	// 	spreadsheetChan := make(chan struct {
	// 	comps   []v1alpha1.ComponentDefinition
	// 	model   string
	// 	helmURL string
	// }, 100)
	var spreadsheetChan chan struct {
    comps   []v1alpha1.ComponentDefinition
    model   string
    helmURL string
}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		utils.Spreadsheet(srv, sheetName, spreadsheetChan, availableModels, availableComponentsPerModel)
	}()
	dp := Newdedup()

	executeInStages(StartPipeline, csvChan, spreadsheetChan, dp, priority, cncf, official, verified, unverified)
	time.Sleep(20 * time.Second)

	close(spreadsheetChan)
	wg.Wait()
}