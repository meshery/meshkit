package policies

import (
	"context"
	"fmt"
	"sync"

	"github.com/meshery/meshkit/models/meshmodel/registry"
	"github.com/meshery/meshkit/models/meshmodel/registry/v1alpha3"
	"github.com/meshery/meshkit/utils"
	patching "github.com/meshery/meshkit/utils/patching"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"github.com/open-policy-agent/opa/v1/rego"
	storagepkg "github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem"
	printpkg "github.com/open-policy-agent/opa/v1/topdown/print"
	"github.com/sirupsen/logrus"
)

var SyncRelationship sync.Mutex

type Rego struct {
	store     storagepkg.Store
	txn       storagepkg.Transaction
	ctx       context.Context
	policyDir string
}

// NewRegoInstance creates a new Rego evaluator with relationships loaded
func NewRegoInstance(policyDir string, regManager *registry.RegistryManager) (*Rego, error) {
	ctx := context.Background()

	registeredRels, _, _, err := regManager.GetEntities(&v1alpha3.RelationshipFilter{})
	if err != nil {
		return nil, err
	}

	// initialize in-memory store with relationships
	data := map[string]interface{}{"relationships": registeredRels}
	store := inmem.NewFromObject(data)
	txn, err := store.NewTransaction(ctx, storagepkg.WriteParams)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	return &Rego{
		store:     store,
		txn:       txn,
		ctx:       ctx,
		policyDir: policyDir,
	}, nil
}

// CustomPrint implements the print.Hook interface to capture print statements
type CustomPrint struct {
	Messages []string
}

func (cp *CustomPrint) Print(ctx printpkg.Context, msg string) error {
	cp.Messages = append(cp.Messages, msg)
	logrus.Info("[OPA] ", msg)
	return nil
}

// ComponentUpdateActionPayload describes patch action from policy
type ComponentUpdateActionPayload struct {
	Id    string      `json:"id"`
	Value interface{} `json:"value"`
	Path  []string    `json:"path"`
}

// RegoPolicyHandler evaluates the given policy query against the design
func (r *Rego) RegoPolicyHandler(
	design pattern.PatternFile,
	query string,
) (pattern.EvaluationResponse, error) {
	var resp pattern.EvaluationResponse
	if r == nil || r.store == nil {
		return resp, fmt.Errorf("policy engine is not initialized")
	}

	// Prepare Rego evaluation
	printHook := &CustomPrint{}
	rg, err := rego.New(
		rego.Query(query),
		rego.Load([]string{r.policyDir}, nil),
		rego.Store(r.store),
		rego.Transaction(r.txn),
		rego.PrintHook(printHook),
		rego.EnablePrintStatements(true),
	).PrepareForEval(r.ctx)
	if err != nil {
		logrus.Error("error preparing Rego evaluation:", err)
		return resp, err
	}

	// Execute evaluation
	evalResult, err := rg.Eval(r.ctx, rego.EvalInput(design))
	if err != nil {
		return resp, err
	}
	if len(evalResult) == 0 || len(evalResult[0].Expressions) == 0 {
		return resp, fmt.Errorf("evaluation returned no results")
	}

	// Extract `evaluate` key
	outMap, err := utils.Cast[map[string]interface{}](evalResult[0].Expressions[0].Value)
	if err != nil {
		return resp, err
	}
	rawEval, ok := outMap["evaluate"]
	if !ok {
		return resp, fmt.Errorf("evaluate key missing in result")
	}

	// Unmarshal into EvaluationResponse
	resp, err = utils.MarshalAndUnmarshal[interface{}, pattern.EvaluationResponse](rawEval)
	if err != nil {
		return resp, err
	}

	// Gather component configuration updates
	var updates []ComponentUpdateActionPayload
	for _, action := range resp.Actions {
		if action.Op == "update_component_configuration" {
			pl, err := utils.MarshalAndUnmarshal[map[string]interface{}, ComponentUpdateActionPayload](action.Value)
			if err != nil {
				logrus.Warn("failed to parse payload:", err)
				continue
			}
			updates = append(updates, pl)
		}
	}

	// Apply patches to components
	for _, comp := range resp.Design.Components {
		var patches []patching.Patch
		for _, up := range updates {
			if up.Id == comp.Id.String() {
				patches = append(patches, patching.Patch{Path: up.Path[1:], Value: up.Value})
			}
		}
		if len(patches) == 0 {
			continue
		}
		updatedConfig, err := patching.ApplyPatches(comp.Configuration, patches)
		if err != nil {
			logrus.Errorf("error applying patches: %v", err)
			continue
		}
		comp.Configuration = updatedConfig
	}

	return resp, nil
}
