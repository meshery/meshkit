package generators

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"path/filepath"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"google.golang.org/api/sheets/v4"
	"github.com/layer5io/meshkit/utils/walker"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/generators/github"
	"github.com/layer5io/meshkit/utils/store"
	"github.com/layer5io/meshkit/utils/registry"
	"golang.org/x/sync/semaphore"
	"github.com/layer5io/meshkit/logger"
)

var (
	srv                      *sheets.Service
	totalAggregateModel      int
	totalAggregateComponents int
	componentSpredsheetGID   int64
	sheetGID                 int64
	registryLocation         string
	logFile                  *os.File
	errorLogFile             *os.File
	Log                      logger.Handler
	LogError                 logger.Handler
)

var (
	artifactHubCount           = 0
	artifactHubRateLimit       = 100
	artifactHubRateLimitDur    = 5 * time.Minute
	artifactHubMutex           sync.Mutex
	defVersion                 = "v1.0.0"
	GoogleSpreadSheetURL       = "https://docs.google.com/spreadsheets/d/"
	logDirPath                 = filepath.Join(utils.GetHome(), ".meshery", "logs", "registry")
	modelToCompGenerateTracker = store.NewGenericThreadSafeStore[registry.CompGenerateTracker]()
)

func InvokeGenerationFromSheet(wg *sync.WaitGroup, spreadsheetID string, spreadsheetCred string) error {

	weightedSem := semaphore.NewWeighted(20)
	url := GoogleSpreadSheetURL + spreadsheetID
	totalAvailableModels := 0
	spreadsheeetChan := make(chan registry.SpreadsheetData)

	defer func() {
		logModelGenerationSummary(modelToCompGenerateTracker)

		Log.UpdateLogOutput(os.Stdout)
		Log.UpdateLogOutput(os.Stdout)
		Log.Info(fmt.Sprintf("Summary: %d models, %d components generated.", totalAggregateModel, totalAggregateComponents))

		Log.Info("See ", logDirPath, " for detailed logs.")

		_ = logFile.Close()
		_ = errorLogFile.Close()
		totalAggregateModel = 0
		totalAggregateComponents = 0
	}()

	modelCSVHelper, err := parseModelSheet(url)
	if err != nil {
		return err
	}

	componentCSVHelper, err := parseComponentSheet(url)
	if err != nil {
		return err
	}

	Log.UpdateLogOutput(logFile)
	Log.UpdateLogOutput(errorLogFile)
	var wgForSpreadsheetUpdate sync.WaitGroup
	wgForSpreadsheetUpdate.Add(1)
	go func() {
		registry.ProcessModelToComponentsMap(componentCSVHelper.Components)
		registry.VerifyandUpdateSpreadsheet(spreadsheetCred, &wgForSpreadsheetUpdate, srv, spreadsheeetChan, spreadsheetID)
	}()

	// Iterate models from the spreadsheet
	for _, model := range modelCSVHelper.Models {
		totalAvailableModels++

		ctx := context.Background()

		err := weightedSem.Acquire(ctx, 1)
		if err != nil {
			break
		}

		wg.Add(1)
		go func(model registry.ModelCSV) {
			defer func() {
				wg.Done()
				weightedSem.Release(1)
			}()
			if utils.ReplaceSpacesAndConvertToLowercase(model.Registrant) == "meshery" {
				err = GenerateDefsForCoreRegistrant(model)
				if err != nil {
					LogError.Error(err)
				}
				return
			}

			generator, err := NewGenerator(model.Registrant, model.SourceURL, model.Model)
			if err != nil {
				LogError.Error(registry.ErrGenerateModel(err, model.Model))
				return
			}

			if utils.ReplaceSpacesAndConvertToLowercase(model.Registrant) == "artifacthub" {
				rateLimitArtifactHub()

			}
			pkg, err := generator.GetPackage()
			if err != nil {
				LogError.Error(registry.ErrGenerateModel(err, model.Model))
				return
			}

			version := pkg.GetVersion()
			modelDirPath, compDirPath, err := CreateVersionedDirectoryForModelAndComp(version, model.Model)
			if err != nil {
				LogError.Error(registry.ErrGenerateModel(err, model.Model))
				return
			}
			modelDef, err := writeModelDefToFileSystem(&model, version, modelDirPath)
			if err != nil {
				LogError.Error(err)
				return
			}

			comps, err := pkg.GenerateComponents()
			if err != nil {
				LogError.Error(registry.ErrGenerateModel(err, model.Model))
				return
			}
			Log.Info("Current model: ", model.Model)
			Log.Info(" extracted ", len(comps), " components for ", model.ModelDisplayName, " (", model.Model, ")")
			for _, comp := range comps {
				comp.Version = defVersion
				if comp.Metadata == nil {
					comp.Metadata = make(map[string]interface{})
				}
				// Assign the component status corresponding to model status.
				// i.e. If model is enabled comps are also "enabled". Ultimately all individual comps itself will have ability to control their status.
				// The status "enabled" indicates that the component will be registered inside the registry.
				comp.Model = *modelDef
				assignDefaultsForCompDefs(&comp, modelDef)
				err := comp.WriteComponentDefinition(compDirPath)
				if err != nil {
					Log.Info(err)
				}
			}

			spreadsheeetChan <- registry.SpreadsheetData{
				Model:      &model,
				Components: comps,
			}

			modelToCompGenerateTracker.Set(model.Model, registry.CompGenerateTracker{
				TotalComps: len(comps),
				Version:    version,
			})
		}(model)

	}
	wg.Wait()
	close(spreadsheeetChan)
	wgForSpreadsheetUpdate.Wait()
	return nil
}

func writeModelDefToFileSystem(model *registry.ModelCSV, version, modelDefPath string) (*v1beta1.Model, error) {
	modelDef := model.CreateModelDefinition(version, defVersion)
	err := modelDef.WriteModelDefinition(modelDefPath+"/model.json", "json")
	if err != nil {
		return nil, err
	}

	return &modelDef, nil
}

func rateLimitArtifactHub() {
	artifactHubMutex.Lock()
	defer artifactHubMutex.Unlock()

	if artifactHubCount > 0 && artifactHubCount%artifactHubRateLimit == 0 {
		Log.Info("Rate limit reached for Artifact Hub. Sleeping for 5 minutes...")
		time.Sleep(artifactHubRateLimitDur)
	}
	artifactHubCount++
}

func logModelGenerationSummary(modelToCompGenerateTracker *store.GenerticThreadSafeStore[registry.CompGenerateTracker]) {
	for key, val := range modelToCompGenerateTracker.GetAllPairs() {
		Log.Info(fmt.Sprintf("Generated %d components for model [%s] %s", val.TotalComps, key, val.Version))
		totalAggregateComponents += val.TotalComps
		totalAggregateModel++
	}

	Log.Info(fmt.Sprintf("-----------------------------\n-----------------------------\nGenerated %d models and %d components", totalAggregateModel, totalAggregateComponents))
}

func parseModelSheet(url string) (*registry.ModelCSVHelper, error) {
	modelCSVHelper, err := registry.NewModelCSVHelper(url, "Models", sheetGID)
	if err != nil {
		return nil, err
	}

	err = modelCSVHelper.ParseModelsSheet(false)
	if err != nil {
		return nil, registry.ErrGenerateModel(err, "unable to start model generation")
	}
	return modelCSVHelper, nil
}

func parseComponentSheet(url string) (*registry.ComponentCSVHelper, error) {
	compCSVHelper, err := registry.NewComponentCSVHelper(url, "Components", componentSpredsheetGID)
	if err != nil {
		return nil, err
	}
	err = compCSVHelper.ParseComponentsSheet()
	if err != nil {
		return nil, registry.ErrGenerateModel(err, "unable to start model generation")
	}
	return compCSVHelper, nil
}

func CreateVersionedDirectoryForModelAndComp(version, modelName string) (string, string, error) {
	modelDirPath := filepath.Join(registryLocation, modelName, version, defVersion)
	err := utils.CreateDirectory(modelDirPath)
	if err != nil {
		return "", "", err
	}

	compDirPath := filepath.Join(modelDirPath, "components")
	err = utils.CreateDirectory(compDirPath)
	return modelDirPath, compDirPath, err
}


func GenerateDefsForCoreRegistrant(model registry.ModelCSV) error {
	totalComps := 0
	var version string
	defer func() {
		modelToCompGenerateTracker.Set(model.Model, registry.CompGenerateTracker{
			TotalComps: totalComps,
			Version:    version,
		})
	}()

	path, err := url.Parse(model.SourceURL)
	if err != nil {
		err = registry.ErrGenerateModel(err, model.Model)
		LogError.Error(err)
		return nil
	}
	gitRepo := github.GitRepo{
		URL:         path,
		PackageName: model.Model,
	}
	owner, repo, branch, root, err := gitRepo.ExtractRepoDetailsFromSourceURL()
	if err != nil {
		err = registry.ErrGenerateModel(err, model.Model)
		LogError.Error(err)
		return nil
	}

	isModelPublished, _ := strconv.ParseBool(model.PublishToRegistry)
	//Initialize walker
	gitWalker := walker.NewGit()
	if isModelPublished {
		gw := gitWalker.
			Owner(owner).
			Repo(repo).
			Branch(branch).
			Root(root).
			RegisterFileInterceptor(func(f walker.File) error {
				// Check if the file has a JSON extension
				if filepath.Ext(f.Name) != ".json" {
					return nil
				}
				contentBytes := []byte(f.Content)
				var componentDef v1beta1.ComponentDefinition
				if err := json.Unmarshal(contentBytes, &componentDef); err != nil {
					return err
				}
				version = componentDef.Model.Model.Version
				modelDirPath, compDirPath, err := CreateVersionedDirectoryForModelAndComp(version, model.Model)
				if err != nil {
					err = registry.ErrGenerateModel(err, model.Model)
					return err
				}
				_, err = writeModelDefToFileSystem(&model, version, modelDirPath) // how to infer this? @Beginner86 any idea? new column?
				if err != nil {
					return registry.ErrGenerateModel(err, model.Model)
				}

				err = componentDef.WriteComponentDefinition(compDirPath)
				if err != nil {
					err = registry.ErrGenerateComponent(err, model.Model, componentDef.DisplayName)
					LogError.Error(err)
				}
				return nil
			})
		err = gw.Walk()
		if err != nil {
			return err
		}
	}

	return nil
}

func assignDefaultsForCompDefs(componentDef *v1beta1.ComponentDefinition, modelDef *v1beta1.Model) {
	componentDef.Metadata["status"] = modelDef.Status
	for k, v := range modelDef.Metadata {
		componentDef.Metadata[k] = v
	}
}
