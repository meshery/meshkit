package v1beta1

import (
	"fmt"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta3/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var applyHelmChartFunc = (*kubernetes.Client).ApplyHelmChart

// kindServed reports whether the cluster already serves the component's kind, which is the
// signal that the operator and CRDs the component depends on are present. Any failure to
// find out counts as "not served", so the install is attempted rather than skipped.
// It is a variable so that tests can run without a cluster.
var kindServed = func(kc *kubernetes.Client, comp component.ComponentDefinition) bool {
	if kc == nil || kc.KubeClient == nil || comp.Component.Version == "" || comp.Component.Kind == "" {
		return false
	}
	list, err := kc.KubeClient.Discovery().ServerResourcesForGroupVersion(comp.Component.Version)
	if err != nil || list == nil {
		return false
	}
	return resourceListServesKind(list, comp.Component.Kind)
}

func resourceListServesKind(list *metav1.APIResourceList, kind string) bool {
	for _, r := range list.APIResources {
		if r.Kind == kind {
			return true
		}
	}
	return false
}

// skipDependentHelmAction reports whether the Helm action for a component's dependents is
// skipped, and the summary to report when it is.
//
// The server calls HandleDependents once per component, and the Helm release is named after
// the chart, so installing again for every component of a model (or on a redeploy) fails with
// "cannot reuse a name that is still in use". The install is therefore skipped when the
// cluster already serves the component's kind, unless an upgrade was requested.
//
// Undeploy never uninstalls the chart. Charts commonly ship their CRDs as templates, so the
// uninstall removes them, and with them every custom resource of those kinds in the cluster,
// including the ones other designs created.
func skipDependentHelmAction(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (string, bool) {
	if !isDeploy {
		return fmt.Sprintf("Left the dependent helm chart for %s installed, it may be in use by other designs", comp.DisplayName), true
	}
	if !performUpgrade && kindServed(kc, comp) {
		return fmt.Sprintf("Dependent resources for %s are already available in the cluster", comp.DisplayName), true
	}
	return "", false
}

func (ah ArtifactHub) HandleDependents(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (summary string, err error) {
	sourceURI, err := resolveSourceURI(comp)
	if err != nil {
		return summary, err
	}
	if sourceURI == "" {
		return fmt.Sprintf("no source_uri declared for %s", comp.DisplayName), nil
	}

	if summary, skip := skipDependentHelmAction(comp, kc, isDeploy, performUpgrade); skip {
		return summary, nil
	}

	namespace := resolveNamespace(comp, "default")

	err = applyHelmChartFunc(kc, kubernetes.ApplyHelmChartConfig{
		URL:                sourceURI,
		Namespace:          namespace,
		CreateNamespace:    true,
		Action:             kubernetes.INSTALL,
		UpgradeIfInstalled: performUpgrade,
	})
	if err != nil {
		return fmt.Sprintf("error deploying dependent helm chart for %s, please procced with manual install or try again", comp.DisplayName), err
	}
	return fmt.Sprintf("Deployed dependent helm chart for %s", comp.DisplayName), nil
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

func (gh GitHub) HandleDependents(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (summary string, err error) {
	sourceURI, err := resolveSourceURI(comp)
	if err != nil {
		return "", err
	}
	if sourceURI == "" {
		return fmt.Sprintf("no source_uri declared for %s", comp.DisplayName), nil
	}

	chartURL, shouldInstall, err := classifyGitHubChartURL(sourceURI)
	if err != nil {
		return "", err
	}
	if !shouldInstall {
		return fmt.Sprintf("No Helm operator dependency found for %s; skipped", comp.DisplayName), nil
	}

	if summary, skip := skipDependentHelmAction(comp, kc, isDeploy, performUpgrade); skip {
		return summary, nil
	}

	fallbackNS := "default"
	if comp.Model != nil && isValidDNS1123Label(comp.Model.Name) {
		fallbackNS = comp.Model.Name
	}
	namespace := resolveNamespace(comp, fallbackNS)

	err = applyHelmChartFunc(kc, kubernetes.ApplyHelmChartConfig{
		URL:                chartURL,
		Namespace:          namespace,
		CreateNamespace:    true,
		Action:             kubernetes.INSTALL,
		UpgradeIfInstalled: performUpgrade,
	})
	if err != nil {
		return fmt.Sprintf("error deploying dependent helm chart for %s, please proceed with manual install or try again: %v", comp.DisplayName, err), err
	}
	return fmt.Sprintf("Deployed dependent helm chart for %s", comp.DisplayName), nil
}

func (gh GitHub) String() string {
	return "github"
}
