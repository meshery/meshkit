package registry

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"gorm.io/gorm"
)

type Host struct {
	ID        uuid.UUID `json:"-"`
	Hostname  string
	Port      int
	Metadata  string
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	IHost     IHost     `gorm:"-"`
}

func createHost(db *database.Handler, h Host) (uuid.UUID, error) {
	h.ID = uuid.New()
	err := db.Create(&h).Error
	return h.ID, err
}

func (h *Host) AfterFind(tx *gorm.DB) error {
	switch h.Hostname {
	case "artifacthub":
		h.IHost = ArtifactHub{}
	case "kubernetes":
		h.IHost = Kubernetes{}
	default:
		return ErrUnknownHost(errors.New("unable to find compatible host for the component"))
	}
	return nil
}

// Each host from where meshmodels can be generated needs to implement this interface
// HandleDependents, contains host specific logic for provisioning required CRDs/operators for corresponding components.
type IHost interface {
	HandleDependents(comp v1alpha1.Component, kc *kubernetes.Client, isDeploy bool) (string, error)
	String() string
}

type ArtifactHub struct{}

func (ah ArtifactHub) HandleDependents(comp v1alpha1.Component, kc *kubernetes.Client, isDeploy bool) (summary string, err error) {
	source_uri := comp.Annotations[fmt.Sprintf("%s.model.source_uri", v1alpha1.MesheryAnnotationPrefix)]
	act := kubernetes.UNINSTALL
	if isDeploy {
		act = kubernetes.INSTALL
	}

	if source_uri != "" {
		err = kc.ApplyHelmChart(kubernetes.ApplyHelmChartConfig{
			URL:                    source_uri,
			Namespace:              comp.Namespace,
			CreateNamespace:        true,
			Action:                 act,
			SkipUpgradeIfInstalled: true,
		})
		if err != nil {
			if !isDeploy {
				summary = fmt.Sprintf("error undeploying dependent resources for %s, please proceed with manual uninstall or try again: %s", strings.TrimSuffix(comp.Spec.Type, ".K8s"), comp.Name)
			} else {
				summary = fmt.Sprintf("error deploying dependent resources for %s, please procced with manual install or try again: %s", strings.TrimSuffix(comp.Spec.Type, ".K8s"), comp.Name)
			}
		} else {
			if !isDeploy {
				summary = fmt.Sprintf("Undeployed helm chart%s: %s", strings.TrimSuffix(comp.Spec.Type, ".K8s"), comp.Name)
			} else {
				summary = fmt.Sprintf("Deployed helm chart%s: %s", strings.TrimSuffix(comp.Spec.Type, ".K8s"), comp.Name)
			}
		}
	}
	return summary, err
}

func (ah ArtifactHub) String() string {
	return "artifacthub"
}

type Kubernetes struct{}

func (k Kubernetes) HandleDependents(comp v1alpha1.Component, kc *kubernetes.Client, isDeploy bool) (summary string, err error) {
	return summary, err
}

func (k Kubernetes) String() string {
	return "kubernetes"
}
