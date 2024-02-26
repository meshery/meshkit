package policies

import "github.com/layer5io/meshkit/errors"

const (
	ErrPrepareForEvalCode = "meshkit-11144"
	ErrEvalCode           = "meshkit-11145"
)

func ErrPrepareForEval(err error) error {
	return errors.New(ErrPrepareForEvalCode, errors.Alert, []string{"error preparing for evaluation"}, []string{err.Error()}, []string{"query might be empty", "rego store provided without associated transaction", "uncommitted transaction"}, []string{"please provide the transaction for the loaded store"})
}

func ErrEval(err error) error {
	return errors.New(ErrEvalCode, errors.Alert, []string{"error evaluating policy for the given input"}, []string{err.Error()}, []string{"The policy query is invalid, see: https://github.com/open-policy-agent/opa/blob/main/rego/resultset.go (Allowed func)"}, []string{"please provide a valid non-empty query"})
}
