package policies

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// RegoPolicyHandler takes the required inputs and run the query against all the policy files provided
func RegoPolicyHandler(ctx context.Context, policyDir []string, regoQueryString string, designFile []byte) (map[string]interface{}, error) {
	regoPolicyLoader := rego.Load(policyDir, nil)

	regoEngine, err := rego.New(
		rego.Query(regoQueryString),
		regoPolicyLoader,
	).PrepareForEval(ctx)
	if err != nil {
		logrus.Error("error preparing for evaluation", err)
	}

	var input map[string]interface{}
	err = yaml.Unmarshal((designFile), &input)
	if err != nil {
		return nil, err
	}

	eval_result, err := regoEngine.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, err
	}

	if !eval_result.Allowed() {
		return eval_result[0].Expressions[0].Value.(map[string]interface{}), nil
	}

	return nil, fmt.Errorf("error evaluation rego reponse, the result is not returning the expressions, see The policy query is invalid: github.com/open-policy-agent/opa@v0.52.0/rego/resultset.go (Allowed func)")
}
