package converter

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrUnknownFormatCode = "meshkit-11245"
)

func ErrUnknownFormat(format DesignFormat) error {
	return errors.New(ErrUnknownFormatCode, errors.Alert, 
		[]string{fmt.Sprintf("The '%s' format is not supported.", format)}, 
		[]string{fmt.Sprintf("Failed to export design in '%s' format.", format)}, 
		[]string{"The format is not supported by the current version of Meshery server."}, 
		[]string{"Please export the design in one of the supported formats."})
}
