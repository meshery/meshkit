package oci

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrAppendingLayerCode           = "meshkit-11147"
	ErrReadingFileCode              = "meshkit-11148"
	ErrUnSupportedLayerTypeCode     = "meshkit-11149"
	ErrGettingLayerCode             = "meshkit-11150"
	ErrCompressingLayerCode         = "meshkit-11151"
	ErrUnTaringLayerCode            = "meshkit-11152"
	ErrGettingImageCode             = "meshkit-11153"
	ErrValidatingImageCode          = "meshkit-11154"
	ErrConnectingToRegistryCode     = "meshkit-11280"
	ErrFileNotFoundCode             = "meshkit-11281"
	ErrAuthenticatingToRegistryCode = "meshkit-11282"
	ErrWriteFilesCode               = "meshkit-11283"
	ErrAddLayerCode                 = "meshkit-11284"
	ErrTaggingPackageCode           = "meshkit-11285"
	ErrPushingPackageCode           = "meshkit-11286"
	ErrSeekFailedCode               = "meshkit-11263"
	ErrCreateLayerCode              = "meshkit-11264"
	ErrSavingImageCode              = "meshkit-11265"
)

func ErrAppendingLayer(err error) error {
	return errors.New(ErrAppendingLayerCode, errors.Alert, []string{"appending content to artifact failed"}, []string{err.Error()}, []string{"layer is not compatible with the base image"}, []string{"Try using a different base image", "use a different media type for the layer"})
}
func ErrSavingImage(err error) error {
	return errors.New(
		ErrSavingImageCode,
		errors.Alert,
		[]string{"Failed to save image"},
		[]string{"An error occurred while saving the image."},
		[]string{"Possible issues with file system, insufficient disk space, , other IO errors."},
		[]string{"Check the file system permissions and available disk space.", "Ensure the file path is correct and accessible.", "Check for any underlying IO errors."},
	)
}
func ErrReadingFile(err error) error {
	return errors.New(ErrReadingFileCode, errors.Alert, []string{"reading file failed"}, []string{err.Error()}, []string{"failed to read the file", "Insufficient permissions"}, []string{"Try using a different file", "check if appropriate read permissions are given to the file"})
}

func ErrUnSupportedLayerType(err error) error {
	return errors.New(ErrUnSupportedLayerTypeCode, errors.Alert, []string{"unsupported layer type"}, []string{err.Error()}, []string{"layer type is not supported"}, []string{"Try using a different layer type", fmt.Sprintf("supported layer types are: %s, %s", LayerTypeTarball, LayerTypeStatic)})
}

func ErrGettingLayer(err error) error {
	return errors.New(ErrGettingLayerCode, errors.Alert, []string{"getting layer failed"}, []string{err.Error()}, []string{"failed to get the layer"}, []string{"Try using a different layer", "check if OCI image is not malformed"})
}

func ErrCompressingLayer(err error) error {
	return errors.New(ErrCompressingLayerCode, errors.Alert, []string{"compressing layer failed"}, []string{err.Error()}, []string{"failed to compress the layer"}, []string{"Try using a different layer", "check if layers are compatible with the base image"})
}
func ErrCreateLayer(err error) error {
	return errors.New(
		ErrCreateLayerCode,
		errors.Alert,
		[]string{"Failed to create layer"},
		[]string{"An error occurred while creating the layer for the image."},
		[]string{"Possible issues with file system, incorrect layer type, or other IO errors."},
		[]string{"Check the file system permissions and paths.", "Ensure the layer type is correct.", "Check for any underlying IO errors."},
	)
}
func ErrUnTaringLayer(err error) error {
	return errors.New(ErrUnTaringLayerCode, errors.Alert, []string{"untaring layer failed"}, []string{err.Error()}, []string{"failed to untar the layer"}, []string{"Try using a different layer", "check if image is not malformed"})
}

func ErrGettingImage(err error) error {
	return errors.New(ErrGettingImageCode, errors.Alert, []string{"getting image failed"}, []string{err.Error()}, []string{"failed to get the image"}, []string{"Try using a different image", "check if image is not malformed"})
}

func ErrValidatingImage(err error) error {
	return errors.New(ErrValidatingImageCode, errors.Alert, []string{"validating image failed"}, []string{err.Error()}, []string{"failed to validate the image"}, []string{"Try using a different image", "check if image is not malformed"})
}

func ErrConnectingToRegistry(err error) error {
	return errors.New(ErrConnectingToRegistryCode, errors.Alert, []string{"connecting to registry failed"}, []string{err.Error()}, []string{"failed to connect to the registry"}, []string{"Try using a different registry", "check if registry URL is correct"})
}

func ErrFileNotFound(err error, filePath string) error {
	return errors.New(ErrFileNotFoundCode, errors.Alert, []string{"file not found at " + filePath}, []string{err.Error()}, []string{"file not found at " + filePath}, []string{"Try using a different file", "check if file exists"})
}

func ErrAuthenticatingToRegistry(err error) error {
	return errors.New(ErrAuthenticatingToRegistryCode, errors.Alert, []string{"authenticating to registry failed"}, []string{err.Error()}, []string{"failed to authenticate to the registry"}, []string{"Please check if the credentials are correct"})
}

func ErrWriteFile(err error) error {
	return errors.New(ErrWriteFilesCode, errors.Alert, []string{"writing file failed"}, []string{err.Error()}, []string{"failed to write the file"}, []string{"Try using a different file", "check if appropriate write permissions are given to the file"})
}

func ErrAddLayer(err error) error {
	return errors.New(ErrAddLayerCode, errors.Alert, []string{"adding file failed"}, []string{err.Error()}, []string{"failed to add the layer"}, []string{"Try using a different file's", "check if layer is compatible with the base image"})
}

func ErrTaggingPackage(err error) error {
	return errors.New(ErrTaggingPackageCode, errors.Alert, []string{"tagging package failed"}, []string{err.Error()}, []string{"failed to tag the package"}, []string{"Try using a different tag", "check if package is not malformed"})
}

func ErrPushingPackage(err error) error {
	return errors.New(ErrPushingPackageCode, errors.Alert, []string{"pushing package failed"}, []string{err.Error()}, []string{"failed to push the package"}, []string{"Try using a different tag", "check if package is not malformed"})
}
func ErrSeekFailed(err error) error {
	return errors.New(ErrSeekFailedCode, errors.Alert, []string{"Unable to reset the position within the OCI data."}, []string{err.Error()}, []string{"The function attempted to move to the start of the data but failed. This could happen if the data is corrupted or not in the expected format."}, []string{"Ensure the input data is a valid OCI archive and try again. Check if the data is compressed correctly and is not corrupted."})
}
