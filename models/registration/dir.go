package registration

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/models/oci"

	meshkitFileUtils "github.com/meshery/meshkit/files"
	"github.com/meshery/meshkit/utils"

	corev1beta1 "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
)

type Dir struct {
	dirpath string
}

/*
The directory should contain one and only one `model`.
A directory containing multiple `model` will be invalid.
*/
func NewDir(path string) Dir {
	return Dir{dirpath: path}
}

/*
PkgUnit parses all the files inside the directory and finds out if they are any valid meshery definitions. Valid meshery definitions are added to the PackagingUnit struct.
Invalid definitions are stored in the regErrStore with error data.
*/
func (d Dir) PkgUnit(regErrStore RegistrationErrorStore) (_ PackagingUnit, err error) {
	pkg := PackagingUnit{}

	// Extract the filename to use as entityName in case of errors
	filename := filepath.Base(d.dirpath)

	// Check if the given path is accessible
	_, err = os.Stat(d.dirpath)
	if err != nil {
		regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filename, ErrDirPkgUnitParseFail(d.dirpath, fmt.Errorf("could not access the path: %w", err)))
		return pkg, ErrDirPkgUnitParseFail(d.dirpath, fmt.Errorf("could not access the path: %w", err))
	}

	// Process the path (file or directory)
	err = processDir(d.dirpath, &pkg, regErrStore)
	if err != nil {
		return pkg, err
	}

	if reflect.ValueOf(pkg.Model).IsZero() {
		errMsg := fmt.Errorf("model definition not found in imported package. Model definitions often use the filename `model.json`, but are not required to have this filename. One and exactly one entity containing schema: model.core must be present, otherwise the model package is considered malformed")
		regErrStore.InsertEntityRegError("", "", entity.Model, filename, errMsg)
		return pkg, errMsg
	}

	return pkg, nil
}

func processDir(dirPath string, pkg *PackagingUnit, regErrStore RegistrationErrorStore) error {
	var tempDirs []string
	defer func() {
		for _, tempDir := range tempDirs {
			utils.SafeRemoveAll(tempDir)
		}
	}()

	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			err = utils.ErrFileWalkDir(fmt.Errorf("error accessing path: %w", err), path)
			regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
			regErrStore.AddInvalidDefinition(path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		// Read the file content
		data, err := os.ReadFile(path)
		if err != nil {
			err = oci.ErrReadingFile(err)
			regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
			regErrStore.AddInvalidDefinition(path, err)
			return nil
		}

		// Check if the file is an OCI artifact
		if oci.IsOCIArtifact(data) {
			// Extract the OCI artifact
			tempDir, err := oci.CreateTempOCIContentDir()
			if err != nil {
				regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
				regErrStore.AddInvalidDefinition(path, err)
				return nil
			}
			tempDirs = append(tempDirs, tempDir)
			err = oci.UnCompressOCIArtifact(path, tempDir)
			if err != nil {
				err := meshkitFileUtils.ErrUnCompressOCIArtifact(err)
				regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
				regErrStore.AddInvalidDefinition(path, err)
				return nil
			}
			// Recursively process the extracted directory
			if err := processDir(tempDir, pkg, regErrStore); err != nil {
				return err
			}
			return nil
		}

		// Check if the file is a zip or tar file
		if utils.IsZip(path) || utils.IsTarGz(path) {
			tempDir, err := os.MkdirTemp("", "nested-extract-")
			if err != nil {
				err = utils.ErrCreateDir(fmt.Errorf("error creating temp directory for nested archive extraction: %w", err), tempDir)
				regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
				regErrStore.AddInvalidDefinition(path, err)
				return nil
			}
			tempDirs = append(tempDirs, tempDir)
			if err := utils.ExtractFile(path, tempDir); err != nil {
				err := meshkitFileUtils.ErrFailedToExtractArchive(filepath.Base(path), err)
				regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
				regErrStore.AddInvalidDefinition(path, err)
				return nil
			}
			// Recursively process the extracted directory
			if err := processDir(tempDir, pkg, regErrStore); err != nil {
				return err
			}
			return nil
		}

		content := data
		if utils.IsYaml(path) {
			content, err = utils.YAMLToJSON(content)
			if err != nil {
				regErrStore.InsertEntityRegError("", "", entity.EntityType("unknown"), filepath.Base(path), err)
				return nil
			}
		}

		// Determine the entity type
		entityType, err := utils.FindEntityType(content)
		if err != nil {
			errMsg := meshkitFileUtils.ErrInvalidModel("import", filepath.Base(path), err)
			regErrStore.InsertEntityRegError("", filepath.Base(path), entity.EntityType("unknown"), filepath.Base(path), errMsg)
			regErrStore.AddInvalidDefinition(path, errMsg)
			return nil
		}

		if entityType == "" {
			// Not an entity we care about
			return nil
		}

		// Get the entity
		var e entity.Entity
		e, err = GetEntity(content)
		if err != nil {
			regErrStore.InsertEntityRegError("", "", entityType, filepath.Base(path), fmt.Errorf("could not get entity: %w", err))
			regErrStore.AddInvalidDefinition(path, fmt.Errorf("could not get entity: %w", err))
			return nil
		}

		// Add the entity to the packaging unit
		switch e.Type() {
		case entity.Model:
			model, err := utils.Cast[*model.ModelDefinition](e)
			if err != nil {
				modelName := ""
				if model != nil {
					modelName = model.Name
				}
				regErrStore.InsertEntityRegError("", modelName, entityType, modelName, ErrGetEntity(err))
				regErrStore.AddInvalidDefinition(path, ErrGetEntity(err))
				return nil
			}
			pkg.Model = *model
		case entity.ComponentDefinition:
			comp, err := utils.Cast[*component.ComponentDefinition](e)
			if err != nil {
				componentName := ""
				if comp != nil {
					componentName = comp.Component.Kind
				}
				regErrStore.InsertEntityRegError("", "", entityType, componentName, ErrGetEntity(err))
				regErrStore.AddInvalidDefinition(path, ErrGetEntity(err))
				return nil
			}
			pkg.Components = append(pkg.Components, *comp)
		case entity.RelationshipDefinition:
			rel, err := utils.Cast[*relationship.RelationshipDefinition](e)
			if err != nil {
				relationshipName := ""
				if rel != nil {
					relationshipName = rel.Model.Name
				}
				regErrStore.InsertEntityRegError("", "", entityType, relationshipName, ErrGetEntity(err))
				regErrStore.AddInvalidDefinition(path, ErrGetEntity(err))
				return nil
			}
			pkg.Relationships = append(pkg.Relationships, *rel)
		case entity.ConnectionDefinition:
			conn, err := utils.Cast[*corev1beta1.ConnectionDefinition](e)
			if err != nil {
				regErrStore.InsertEntityRegError("", "", entityType, "", ErrGetEntity(err))
				regErrStore.AddInvalidDefinition(path, ErrGetEntity(err))
				return nil
			}
			pkg.Connections = append(pkg.Connections, *conn)
		default:
			// Unhandled entity type
			return nil
		}
		return nil
	})
}
