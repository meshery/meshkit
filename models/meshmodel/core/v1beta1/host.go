package v1beta1

import (
	"fmt"

	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
)

type MeshModelHostsWithEntitySummary struct {
	connection.Connection
	Summary EntitySummary `json:"summary"`
}
type EntitySummary struct {
	Models        int64 `json:"models"`
	Components    int64 `json:"components"`
	Relationships int64 `json:"relationships"`
	Policies      int64 `json:"policies"`
}
type MesheryHostSummaryDB struct {
	connection.Connection
	Models        int64 `json:"-" gorm:"models"`
	Components    int64 `json:"-" gorm:"components"`
	Relationships int64 `json:"-" gorm:"relationships"`
	Policies      int64 `json:"-" gorm:"policies"`
}

type HostFilter struct {
	Name        string
	Greedy      bool //when set to true - instead of an exact match, name will be prefix matched
	Trim        bool //when set to true - the schema is not returned
	DisplayName string
	Version     string
	Sort        string //asc or desc. Default behavior is asc
	OrderOn     string
	Limit       int //If 0 or  unspecified then all records are returned and limit is not used
	Offset      int
}

type DependencyHandler interface {
	HandleDependents(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (string, error)
	String() string
}

func NewDependencyHandler(connectionKind string) (DependencyHandler, error) {
	switch connectionKind {
	case "kubernetes":
		return Kubernetes{}, nil
	case "artifacthub":
		return ArtifactHub{}, nil
	case "github":
		return GitHub{}, nil
	}
	return nil, ErrUnknownKind(fmt.Errorf("unknown kind %s", connectionKind))
}

// Each connection from where meshmodels can be generated needs to implement this interface
// HandleDependents, contains host specific logic for provisioning required CRDs/operators for corresponding components.

type ArtifactHub struct{}

const MesheryAnnotationPrefix = "design.meshery.io"

func (ah ArtifactHub) HandleDependents(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (summary string, err error) {
	sourceURI, ok := comp.Model.Metadata.AdditionalProperties["source_uri"] // should be part of registrant data(?)
	if !ok {
		return summary, err
	}

	act := kubernetes.UNINSTALL
	if isDeploy {
		act = kubernetes.INSTALL
	}

	_sourceURI, err := utils.Cast[string](sourceURI)
	if err != nil {
		return summary, err
	}

	if sourceURI != "" {
		err = kc.ApplyHelmChart(kubernetes.ApplyHelmChartConfig{
			URL:                _sourceURI,
			Namespace:          comp.Configuration["namespace"].(string),
			CreateNamespace:    true,
			Action:             act,
			UpgradeIfInstalled: performUpgrade,
		})
		if err != nil {
			if !isDeploy {
				summary = fmt.Sprintf("error undeploying dependent helm chart for %s, please proceed with manual uninstall or try again", comp.DisplayName)
			} else {
				summary = fmt.Sprintf("error deploying dependent helm chart for %s, please procced with manual install or try again", comp.DisplayName)
			}
		} else {
			if !isDeploy {
				summary = fmt.Sprintf("Undeployed dependent helm chart for %s", comp.DisplayName)
			} else {
				summary = fmt.Sprintf("Deployed dependent helm chart for %s", comp.DisplayName)
			}
		}
	}
	return summary, err
}

func (ah ArtifactHub) String() string {
	return "artifacthub"
}

type Kubernetes struct{}

func (k Kubernetes) HandleDependents(_ component.ComponentDefinition, _ *kubernetes.Client, _, _ bool) (summary string, err error) {
	return summary, err
}

func (k Kubernetes) String() string {
	return "kubernetes"
}

type GitHub struct{}

func(gh GitHub) HandleDependents(_ component.ComponentDefinition, _ *kubernetes.Client, _, _ bool) (summary string, err error) {
	return summary, err
}

func(gh GitHub) String() string {
	return "github"
}