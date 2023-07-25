package describe

import "github.com/layer5io/meshkit/errors"

var (
	ErrGetDescriberFuncCode = "11096"
)

func ErrGetDescriberFunc() error {
	return errors.New(
		ErrGetDescriberFuncCode,
		errors.Fatal,
		[]string{"Failed to get describer for the resource"},
		[]string{"invalid kubernetes object type or object type not supported in meshkit", "Describer not found for the defined Resource"},
		nil, nil,
	)
}
