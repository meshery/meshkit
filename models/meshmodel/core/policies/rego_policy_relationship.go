package policies

import (
	"context"
	"fmt"
	"sync"

	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/layer5io/meshkit/models/meshmodel/registry/v1alpha3"
	"github.com/layer5io/meshkit/utils"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"github.com/open-policy-agent/opa/rego"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/sirupsen/logrus"
)

var SyncRelationship sync.Mutex

type Rego struct {
	store       storage.Store
	ctx         context.Context
	transaction storage.Transaction
	policyDir   string
}

func NewRegoInstance(policyDir string, regManager *registry.RegistryManager) (*Rego, error) {
	var txn storage.Transaction
	var store storage.Store

	ctx := context.Background()
	registeredRelationships, _, _, err := regManager.GetEntities(&v1alpha3.RelationshipFilter{})
	if err != nil {
		return nil, err
	}

	if len(registeredRelationships) > 0 {
		data := map[string]interface{}{
			"relationships": registeredRelationships,
		}
		store = inmem.NewFromObject(data)
		txn, _ = store.NewTransaction(ctx, storage.WriteParams)

	}
	return &Rego{
		store:       store,
		ctx:         ctx,
		transaction: txn,
		policyDir:   policyDir,
	}, nil
}

// RegoPolicyHandler takes the required inputs and run the query against all the policy files provided
func (r *Rego) RegoPolicyHandler(designFile pattern.PatternFile, regoQueryString string, relationshipsToEvalaute ...string) (pattern.EvaluationResponse, error) {
	var evaluationResponse pattern.EvaluationResponse
	if r == nil {
		return evaluationResponse, ErrEval(fmt.Errorf("policy engine is not yet ready"))
	}
	regoEngine, err := rego.New(
		rego.Query(regoQueryString),
		rego.Load([]string{r.policyDir}, nil),
		rego.Store(r.store),
		rego.Transaction(r.transaction),
	).PrepareForEval(r.ctx)
	if err != nil {
		logrus.Error("error preparing for evaluation", err)
		return evaluationResponse, ErrPrepareForEval(err)
	}

	eval_result, err := regoEngine.Eval(r.ctx, rego.EvalInput(designFile))
	if err != nil {
		return evaluationResponse, ErrEval(err)
	}

	if len(eval_result) == 0 {
		return evaluationResponse, ErrEval(fmt.Errorf("evaluation results are empty"))
	}

	if len(eval_result[0].Expressions) == 0 {
		return evaluationResponse, ErrEval(fmt.Errorf("evaluation results are empty"))
	}

	result, err := utils.Cast[map[string]interface{}](eval_result[0].Expressions[0].Value)
	if err != nil {
		return evaluationResponse, ErrEval(err)
	}

	evalResults, ok := result["evaluate"]
	if !ok {
		return evaluationResponse, ErrEval(fmt.Errorf("evaluation results are empty"))
	}

	evaluationResponse, err = utils.MarshalAndUnmarshal[interface{}, pattern.EvaluationResponse](evalResults)
	if err != nil {
		return evaluationResponse, err
	}

	return evaluationResponse, nil

}
