package policies

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
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

func NewRegoInstance(policyDir string, relationshipDir string) (*Rego, error) {
	var relationships []v1alpha1.RelationshipDefinition
	ctx := context.Background()

	err := filepath.Walk(relationshipDir, func(path string, info fs.FileInfo, err error) error {
		var relationship v1alpha1.RelationshipDefinition
		if !info.IsDir() {
			byt, err := os.ReadFile(path)
			if err != nil {
				return utils.ErrReadingLocalFile(err)
			}
			err = json.Unmarshal(byt, &relationship)
			if err != nil {
				return utils.ErrUnmarshal(err)
			}
			relationships = append(relationships, relationship)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	data := mapRelationshipsWithSubType(&relationships)
	store := inmem.NewFromObject(data)
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)

	return &Rego{
		store:       store,
		ctx:         ctx,
		transaction: txn,
		policyDir:   policyDir,
	}, nil
}

func mapRelationshipsWithSubType(relationships *[]v1alpha1.RelationshipDefinition) map[string]interface{} {
	relMap := make(map[string]interface{}, len(*relationships))
	for _, relationship := range *relationships {
		relMap[strings.ToLower(relationship.SubType)] = relationship
	}
	return relMap
}

// RegoPolicyHandler takes the required inputs and run the query against all the policy files provided
func (r *Rego) RegoPolicyHandler(regoQueryString string, designFile []byte) (interface{}, error) {
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
