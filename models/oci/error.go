package oci

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrAppendingLayerCode       = "11107"
	ErrReadingFileCode          = "11108"
	ErrUnSupportedLayerTypeCode = "11109"
	ErrGettingLayerCode         = "11110"
	ErrCompressingLayerCode     = "11111"
	ErrUnTaringLayerCode        = "11112"
)

func ErrAppendingLayer(err error) error {
	return errors.New(ErrAppendingLayerCode, errors.Alert, []string{"appending content to artifact failed"}, []string{err.Error()}, []string{"layer is not compatible with the base image"}, []string{"Try using a different base image", "use a different media type for the layer"})
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

func ErrUnTaringLayer(err error) error {
	return errors.New(ErrUnTaringLayerCode, errors.Alert, []string{"untaring layer failed"}, []string{err.Error()}, []string{"failed to untar the layer"}, []string{"Try using a different layer", "check if image is not malformed"})
}
