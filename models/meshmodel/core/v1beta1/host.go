package v1beta1

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta1"
)

type MeshModelHostsWithEntitySummary struct {
	ID       uuid.UUID     `json:"id"`
	Hostname string        `json:"hostname"`
	Port     int           `json:"port"`
	Summary  EntitySummary `json:"summary"`
}
type EntitySummary struct {
	Models        int64 `json:"models"`
	Components    int64 `json:"components"`
	Relationships int64 `json:"relationships"`
	Policies      int64 `json:"policies"`
}
type MesheryHostSummaryDB struct {
	HostID        uuid.UUID `json:"-" gorm:"id"`
	Hostname      string    `json:"-" gorm:"hostname"`
	Port          int       `json:"-" gorm:"port"`
	Models        int64     `json:"-" gorm:"models"`
	Components    int64     `json:"-" gorm:"components"`
	Relationships int64     `json:"-" gorm:"relationships"`
	Policies      int64     `json:"-" gorm:"policies"`
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
	HandleDependents(comp v1beta1.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (string, error)
	String() string
}

func NewDependencyHandler(connectionKind string) (DependencyHandler, error) {
	switch connectionKind {
	case "kubernetes":
		return Kubernetes{}, nil
	case "artifacthub":
		return ArtifactHub{}, nil
	}
	return nil, ErrUnknownKind(fmt.Errorf("unknown kind %s", connectionKind))
}

// Each connection from where meshmodels can be generated needs to implement this interface
// HandleDependents, contains host specific logic for provisioning required CRDs/operators for corresponding components.

type ArtifactHub struct{}

const MesheryAnnotationPrefix = "design.meshmodel.io"

func (ah ArtifactHub) HandleDependents(comp v1beta1.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (summary string, err error) {
	sourceURI, err := utils.Cast[string](comp.Metadata.AdditionalProperties["source_uri"]) // should be part of registrant data(?)
	if err != nil {
		return summary, err
	}

	act := kubernetes.UNINSTALL
	if isDeploy {
		act = kubernetes.INSTALL
	}

	if sourceURI != "" {
		err = kc.ApplyHelmChart(kubernetes.ApplyHelmChartConfig{
			URL:                sourceURI,
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

func (k Kubernetes) HandleDependents(_ v1beta1.ComponentDefinition, _ *kubernetes.Client, _, _ bool) (summary string, err error) {
	return summary, err
}

func (k Kubernetes) String() string {
	return "kubernetes"
}
