package kubernetes

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/layer5io/meshkit/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

// HelmDriver is the type for helm drivers
type HelmDriver string

const (
	// ConfigMap HelmDriver can be used to instruct
	// helm to use configmaps as backend
	ConfigMap HelmDriver = "configmap"

	// Secret HelmDriver can be used to instruct
	// helm to use secrets as backend
	Secret HelmDriver = "secret"

	// SQL HelmDriver can be used to instruct
	// helm to use sql as backend
	//
	// This should be used when release information
	// is expected to be greater than 1MB
	SQL HelmDriver = "sql"
)

const (
	// Stable is the default repository for helm v3
	Stable = "https://charts.helm.sh/stable"

	// Latest is the default version for helm charts
	Latest = ">0.0.0-0"
)

var (
	// downloadLocaton is the location where downloaded helm charts
	// will be stored. os.TempDir will ensure that the path is cross
	// platform
	downloadLocation = os.TempDir()
)

// HelmChartLocation describes the structure for defining
// the location for helm chart
type HelmChartLocation struct {
	// Repository is the url of the helm repository
	//
	// Defaults to https://charts.helm.sh/stable
	Repository string

	// Chart is the name of the chart that is supposed
	// to be installed. This chart must me present in the
	// https://REPOSITORY/index.yaml
	Chart string

	// Version is the chart version. This version
	// must be present in the https://REPOSITORY/index.yaml
	//
	// Defaults to Latest
	Version string
}

// ApplyHelmChartConfig defines the options that ApplyHelmChart
// can take
type ApplyHelmChartConfig struct {
	// ChartLocation is the remote location of the helm chart
	//
	// Either ChartLocation or URL can be defined, if both of them
	// are defined then URL is given the preferenece
	ChartLocation HelmChartLocation

	// URL is the url for charts
	//
	// Either ChartLocation or URL can be defined, if both of them
	// are defined then URL is given the preferenece
	URL string

	// HelmDriver is used to determine the backend
	// informations used by helm for managing release
	//
	// Defaults to Secret
	HelmDriver HelmDriver

	// SQLConnectionString is the connection uri
	// for the postgresql database which will be used if
	// the HelmDriver is set to SQL
	SQLConnectionString string

	// Namespace in which the resources are supposed to
	// be deployed
	//
	// Defaults to "default"
	Namespace string

	// CreateNamespace creates namespace if it doesn't exists
	//
	// Defaults to false
	CreateNamespace bool

	// OverrideValues are used during installation
	// to override the the values present in Values.yaml
	// it is equivalent to --set or --set-file helm flag
	OverrideValues map[string]interface{}

	// Delete indicates if the requested action is a delete
	// action
	//
	// Defaults to false
	Delete bool
}

// ApplyHelmChart takes in the url for the helm chart
// and applies that chart as per the ApplyHelmChartOptions
//
// The Helm library requires the environment variable KUBECONFIG to be set.
//
// ApplyHelmChart supports:
//
// - Installation and uninstallation of charts.
//
// - All storage drivers.
//
// - Chart location as a url as well as in form of repository (url) and chart name.
//
// - Override values (equivalent to --set, --set-file, --values in helm).
//
// Examples:
//
// Install Traefik Mesh using URL:
//    err = client.ApplyHelmChart(k8s.ApplyHelmChartConfig{
//            Namespace:       "traefik-mesh",
//            CreateNamespace: true,
//            URL:             "https://helm.traefik.io/mesh/traefik-mesh-3.0.6.tgz",
//    })
//
// Install Traefik Mesh using repository:
//    err = cl.ApplyHelmChart(k8s.ApplyHelmChartConfig{
//            ChartLocation: k8s.HelmChartLocation{
//                Repository: "https://helm.traefik.io/mesh",
//                Chart:      "traefik-mesh",
//            },
//            Namespace:       "traefik-mesh",
//            CreateNamespace: true,
//    })
//
// Install Consul Service Mesh overriding values using a values file (equivalent to -f/--values in helm):
//
//	p := getter.All(cli.New())
//	valueOpts := &values.Options{}
//	if valuesFile, ok := operation.AdditionalProperties[config.HelmChartValuesFileKey]; ok {
//		valueOpts.ValueFiles = []string{path.Join("consul", "config_templates", valuesFile)}
//	}
//	vals, err := valueOpts.MergeValues(p)
//
//	err = kubeClient.ApplyHelmChart(mesherykube.ApplyHelmChartConfig{
//		Namespace:       request.Namespace,
//		CreateNamespace: true,
//		Delete:          request.IsDeleteOperation,
//		ChartLocation: mesherykube.HelmChartLocation{
//			Repository: operation.AdditionalProperties[config.HelmChartRepositoryKey],
//			Chart:      operation.AdditionalProperties[config.HelmChartChartKey],
//			Version:    operation.AdditionalProperties[config.HelmChartVersionKey],
//		},
//		OverrideValues: vals,
//	})
//
func (client *Client) ApplyHelmChart(cfg ApplyHelmChartConfig) error {
	setupDefaults(&cfg)

	url, err := getHelmChartURL(cfg)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	localPath, err := fetchHelmChart(url)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	chart, err := loader.Load(localPath)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	if err := checkIfInstallable(chart); err != nil {
		return ErrApplyHelmChart(err)
	}

	actionConfig, err := createHelmActionConfig(client.RestConfig, cfg)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	if err := generateAction(actionConfig, cfg)(chart); err != nil {
		return ErrApplyHelmChart(err)
	}

	return nil
}

// setupDefaults adds the default value to the configuration
func setupDefaults(cfg *ApplyHelmChartConfig) {
	if cfg.URL == "" {
		if cfg.ChartLocation.Repository == "" {
			cfg.ChartLocation.Repository = Stable
		}
		if cfg.ChartLocation.Version == "" {
			cfg.ChartLocation.Version = Latest
		}
	}
	if cfg.HelmDriver == "" {
		cfg.HelmDriver = Secret
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
}

// getHelmChartURL returns the chart url irrespective of the chosen method for
// performing action
func getHelmChartURL(cfg ApplyHelmChartConfig) (string, error) {
	if cfg.URL == "" {
		return createHelmPathFromHelmChartLocation(cfg.ChartLocation)
	}

	return cfg.URL, nil
}

// fetchHelmChart downloads the charts from the given url and returns
// the location of the downloaded chart
//
// if the chart is already present in the download location
// then the download is skipped
func fetchHelmChart(chartURL string) (string, error) {
	filename := filepath.Base(chartURL)
	downloadPath := path.Join(downloadLocation, filename)

	// Skip the download if chart already exists
	if _, err := os.Stat(downloadPath); err == nil {
		return downloadPath, nil
	}

	if err := utils.DownloadFile(downloadPath, chartURL); err != nil {
		return "", ErrApplyHelmChart(err)
	}

	return downloadPath, nil
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return ErrApplyHelmChart(fmt.Errorf("%s charts are not installable", ch.Metadata.Type))
}

// createHelmActionConfig generates the actionConfig with the appropriate defaults
func createHelmActionConfig(restConfig rest.Config, cfg ApplyHelmChartConfig) (*action.Configuration, error) {
	// Set the environment variable needed by the Init method
	os.Setenv("HELM_DRIVER_SQL_CONNECTION_STRING", cfg.SQLConnectionString)

	// KubeConfig setup
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &restConfig.Host
	kubeConfig.BearerToken = &restConfig.BearerToken
	kubeConfig.CAFile = &restConfig.CAFile

	nopLogger := func(_ string, _ ...interface{}) {} // Dummy logger for helm packages

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kubeConfig, cfg.Namespace, string(cfg.HelmDriver), nopLogger); err != nil {
		return nil, ErrApplyHelmChart(err)
	}

	return actionConfig, nil
}

// generateAction generates an action function using action.Configuration
// and ApplyHelmChartConfig and returns it
//
// The intention is to create a factory function which creates a layer of abstraction
// on top of helm actions making them follow the same interface, hence easing extending
// the number of supported helm actions
func generateAction(actionConfig *action.Configuration, cfg ApplyHelmChartConfig) func(*chart.Chart) error {
	if cfg.Delete {
		return func(c *chart.Chart) error {
			act := action.NewUninstall(actionConfig)
			if _, err := act.Run(c.Name()); err != nil {
				return ErrApplyHelmChart(err)
			}

			return nil
		}
	}

	return func(c *chart.Chart) error {
		act := action.NewInstall(actionConfig)
		act.ReleaseName = c.Name()
		act.CreateNamespace = cfg.CreateNamespace
		act.Namespace = cfg.Namespace
		if _, err := act.Run(c, cfg.OverrideValues); err != nil {
			return ErrApplyHelmChart(err)
		}

		return nil
	}
}

// createHelmPathFromHelmChartLocation takes in the HelmChartLocation and returns the
// chart url which can be used to download the chart
func createHelmPathFromHelmChartLocation(loc HelmChartLocation) (string, error) {
	if loc.Chart == "" {
		return "", ErrApplyHelmChart(fmt.Errorf("\"Chart\" cannot be empty"))
	}

	chartURL, err := repo.FindChartInRepoURL(loc.Repository, loc.Chart, loc.Version, "", "", "", getter.Providers{
		getter.Provider{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		}},
	)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}

	return chartURL, nil
}
