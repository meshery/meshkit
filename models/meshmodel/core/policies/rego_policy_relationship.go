package policies

import (
	"context"
	"fmt"
	"sync"

	"github.com/meshery/meshkit/models/meshmodel/registry"
	"github.com/meshery/meshkit/models/meshmodel/registry/v1alpha3"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/patching"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown/print"
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

// CustomPrintHook implements the print.Hook interface
type CustomPrintHook struct {
	Messages []string
}

// Print captures print messages from policy evaluation
// Implements print.Hook interface
func (h *CustomPrintHook) Print(ctx print.Context, s string) error {
	h.Messages = append(h.Messages, s)
	logrus.Info("[OPA] ", s)
	return nil
}

type ComponentUpdateActionPayload struct {
	Id    string      `json:"id"`
	Value interface{} `json:"value"`
	Path  []string    `json:"path"`
}

// RegoPolicyHandler takes the required inputs and run the query against all the policy files provided
func (r *Rego) RegoPolicyHandler(designFile pattern.PatternFile, regoQueryString string, relationshipsToEvalaute ...string) (pattern.EvaluationResponse, error) {
	var evaluationResponse pattern.EvaluationResponse
	if r == nil {
		return evaluationResponse, ErrEval(fmt.Errorf("policy engine is not yet ready"))
	}
	// Create custom print hook
	printHook := &CustomPrintHook{
		Messages: make([]string, 0),
	}
	regoEngine, err := rego.New(
		rego.PrintHook(printHook),
		rego.EnablePrintStatements(true), // Explicitly enable print statements
		rego.Transaction(r.transaction),
		rego.Query(regoQueryString),
		rego.Load([]string{r.policyDir}, nil),
		rego.Store(r.store),
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

	componentConfigurationUpdates := []ComponentUpdateActionPayload{}

	for _, action := range evaluationResponse.Actions {
		if action.Op == "update_component_configuration" {
			payload, err := utils.MarshalAndUnmarshal[map[string]interface{}, ComponentUpdateActionPayload](action.Value)

			if err != nil {
				fmt.Println("failed to parse payload", err)
				continue
			}
			componentConfigurationUpdates = append(componentConfigurationUpdates, payload)
		}
	}

	// apply patches to components
	for _, component := range evaluationResponse.Design.Components {

		componentPatches := []patch.Patch{}

		for _, payload := range componentConfigurationUpdates {

			if payload.Id == component.Id.String() {

				// remove "configuration" i.e first index from path as we are directly updating that
				componentPatches = append(componentPatches, patch.Patch{
					Path:  payload.Path[1:],
					Value: payload.Value,
				})
			}
		}

		if len(componentPatches) == 0 {
			continue
		}

		updated, err := patch.ApplyPatches(component.Configuration, componentPatches)

		if err != nil {
			fmt.Println(fmt.Errorf("error patching %v", err))
		} else {
			component.Configuration = updated
		}

	}

	return evaluationResponse, nil

}
