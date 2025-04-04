package converter

import (
	"github.com/layer5io/meshkit/converter"
)

type ConvertFormat interface {
	Convert(string) (string, error)
}

func NewFormatConverter(format DesignFormat) (ConvertFormat, error) {
	switch format {
	case K8sManifest:
		return &converter.K8sConverter{}, nil
	case HelmChart:
		// fmt.Println("HelmChart format detected, creating HelmConverter")
		return &converter.HelmConverter{}, nil
	default:
		return nil, ErrUnknownFormat(format)
	}
}
