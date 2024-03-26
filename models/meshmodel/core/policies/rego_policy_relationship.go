package policies

import (
	"context"
	"fmt"

	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/layer5io/meshkit/utils"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

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
	registeredRelationships, _, _, err := regManager.GetEntities(&registry.RelationshipFilter{})
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
func (r *Rego) RegoPolicyHandler(regoQueryString string, designFile []byte) (interface{}, error) {
	if r == nil {
		return nil, ErrEval(fmt.Errorf("policy engine is not yet ready"))
	}
	regoEngine, err := rego.New(
		rego.Query(regoQueryString),
		rego.Load([]string{r.policyDir}, nil),
		rego.Store(r.store),
		rego.Transaction(r.transaction),
	).PrepareForEval(r.ctx)
	if err != nil {
		logrus.Error("error preparing for evaluation", err)
		return nil, ErrPrepareForEval(err)
	}

	var input map[string]interface{}
	err = yaml.Unmarshal((designFile), &input)
	if err != nil {
		return nil, utils.ErrUnmarshal(err)
	}

	eval_result, err := regoEngine.Eval(r.ctx, rego.EvalInput(input))
	if err != nil {
		return nil, ErrEval(err)
	}

	if !eval_result.Allowed() {
		if len(eval_result) > 0 {
			if len(eval_result[0].Expressions) > 0 {
				return eval_result[0].Expressions[0].Value, nil
			}
			return nil, ErrEval(fmt.Errorf("evaluation results are empty"))
		}
		return nil, ErrEval(fmt.Errorf("evaluation results are empty"))
	}

	return nil, ErrEval(err)
}
