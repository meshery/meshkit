package policies

import "github.com/layer5io/meshkit/errors"

const (
	ErrPrepareForEvalCode = "meshkit-11144"
	ErrEvalCode           = "meshkit-11145"
)

func ErrPrepareForEval(err error) error {
	return errors.New(ErrPrepareForEvalCode, errors.Alert, 
		[]string{"Error preparing for evaluation."}, 
		[]string{err.Error()}, 
		[]string{
			"Query might be empty.",
			"Rego store provided without associated transaction.",
			"Uncommitted transaction.",
		}, 
		[]string{"Please provide the transaction for the loaded store."})
}

func ErrEval(err error) error {
	return errors.New(ErrEvalCode, errors.Alert, 
		[]string{"Error evaluating policy for the given input."}, 
		[]string{err.Error()}, 
		[]string{"The policy query is invalid. See: https://github.com/open-policy-agent/opa/blob/main/rego/resultset.go (Allowed func)."}, 
		[]string{"Please provide a valid non-empty query."})
}
