package v1beta1

import (
	"fmt"
	"strings"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta3/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// installChartOnceFunc installs a chart unless its Helm release is already there. It is a
// variable so that tests can run without a cluster.
var installChartOnceFunc = (*kubernetes.Client).InstallHelmChartOnce

// kindServed reports whether the cluster already serves the component's kind, which is the
// signal that the CRDs the component depends on are present. It covers operators that were
// installed without Helm, where there is no release to find. Built-in kinds (a group without a
// dot, such as the core group or apps) are always served and say nothing about the operator, so
// they never count. Any failure to find out counts as "not served".
// It is a variable so that tests can run without a cluster.
var kindServed = func(kc *kubernetes.Client, comp component.ComponentDefinition) bool {
	if kc == nil || kc.KubeClient == nil || comp.Component.Version == "" || comp.Component.Kind == "" {
		return false
	}
	gv, err := schema.ParseGroupVersion(comp.Component.Version)
	if err != nil || !strings.Contains(gv.Group, ".") {
		return false
	}
	list, err := kc.KubeClient.Discovery().ServerResourcesForGroupVersion(comp.Component.Version)
	if err != nil || list == nil {
		return false
	}
	return resourceListServesKind(list, comp.Component.Kind)
}

// resourceListServesKind reports whether the API resource list contains the given kind.
func resourceListServesKind(list *metav1.APIResourceList, kind string) bool {
	for _, r := range list.APIResources {
		if r.Kind == kind {
			return true
		}
	}
	return false
}

// applyDependentHelmChart runs the Helm action for a component's dependents and returns the
// summary to report.
//
// The server calls HandleDependents once per component, and the Helm release is named after
// the chart, so installing for every component of a model (or again on a redeploy) fails with
// "cannot reuse a name that is still in use". On deploy the chart is therefore installed
// through InstallHelmChartOnce, which skips it when the release is already deployed or, for an
// operator installed without Helm, when the cluster already serves the component's kind, and
// returns an error when the release exists in a state that a new install cannot fix. A deployed
// release at another chart version is left alone and reported as such. An explicit upgrade always
// runs.
//
// Undeploy never uninstalls the chart. Charts commonly ship their CRDs as templates, so the
// uninstall removes them, and with them every custom resource of those kinds in the cluster,
// including the ones other designs created. Removing a chart is left to an explicit,
// administrator-controlled step.
func applyDependentHelmChart(comp component.ComponentDefinition, kc *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig, isDeploy, performUpgrade bool) (string, error) {
	if !isDeploy {
		return fmt.Sprintf("Left the dependent helm chart for %s installed, it may be in use by other designs", comp.DisplayName), nil
	}
	if performUpgrade {
		if err := applyHelmChartFunc(kc, cfg); err != nil {
			return "", err
		}
		return fmt.Sprintf("Deployed dependent helm chart for %s", comp.DisplayName), nil
	}
	outcome, err := installChartOnceFunc(kc, cfg, func() bool { return kindServed(kc, comp) })
	if err != nil {
		return "", err
	}
	switch outcome {
	case kubernetes.HelmAlreadyInstalled:
		return fmt.Sprintf("Dependent helm chart for %s is already installed", comp.DisplayName), nil
	case kubernetes.HelmAlreadyInstalledOtherVersion:
		return fmt.Sprintf("Dependent helm chart for %s is already installed at a different chart version than the model declares; run an explicit upgrade to change it", comp.DisplayName), nil
	case kubernetes.HelmProvidedElsewhere:
		return fmt.Sprintf("Dependent resources for %s are already available in the cluster", comp.DisplayName), nil
	}
	return fmt.Sprintf("Deployed dependent helm chart for %s", comp.DisplayName), nil
}

func (ah ArtifactHub) HandleDependents(comp component.ComponentDefinition, kc *kubernetes.Client, isDeploy, performUpgrade bool) (summary string, err error) {
	sourceURI, err := resolveSourceURI(comp)
	if err != nil {
		return summary, err
	}
	if sourceURI == "" {
		return fmt.Sprintf("no source_uri declared for %s", comp.DisplayName), nil
	}

	cfg := kubernetes.ApplyHelmChartConfig{
		URL:                sourceURI,
		Namespace:          resolveNamespace(comp, "default"),
		CreateNamespace:    true,
		Action:             kubernetes.INSTALL,
		UpgradeIfInstalled: performUpgrade,
	}
	summary, err = applyDependentHelmChart(comp, kc, cfg, isDeploy, performUpgrade)
	if err != nil {
		return fmt.Sprintf("error deploying dependent helm chart for %s, please proceed with manual install or try again", comp.DisplayName), err
	}
	return summary, nil
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

	fallbackNS := "default"
	if comp.Model != nil && isValidDNS1123Label(comp.Model.Name) {
		fallbackNS = comp.Model.Name
	}
	cfg := kubernetes.ApplyHelmChartConfig{
		URL:                chartURL,
		Namespace:          resolveNamespace(comp, fallbackNS),
		CreateNamespace:    true,
		Action:             kubernetes.INSTALL,
		UpgradeIfInstalled: performUpgrade,
	}
	summary, err = applyDependentHelmChart(comp, kc, cfg, isDeploy, performUpgrade)
	if err != nil {
		return fmt.Sprintf("error deploying dependent helm chart for %s, please proceed with manual install or try again: %v", comp.DisplayName, err), err
	}
	return summary, nil
}

func (gh GitHub) String() string {
	return "github"
}
