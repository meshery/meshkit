package registry

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
)

type SpreadsheetData struct {
	Model      *ModelCSV
	Components []v1beta1.ComponentDefinition
}

type CompGenerateTracker struct {
	TotalComps int
	Version    string
}
