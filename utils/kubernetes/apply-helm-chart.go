package kubernetes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/utils"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// HelmDriver is the type for helm drivers
type HelmDriver string

// HelmChartAction is the type for helm chart actions
type HelmChartAction int64

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
	INSTALL HelmChartAction = iota
	UPGRADE
	UNINSTALL
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

// HelmIndex holds the index.yaml data in the struct format
type HelmIndex struct {
	APIVersion string      `yaml:"apiVersion"`
	Entries    HelmEntries `yaml:"entries"`
}

// HelmEntries holds the data for all of the entries present
// in the helm repository
type HelmEntries map[string][]HelmEntryMetadata

// HelmEntryMetadata is the struct for holding the metadata
// associated with a helm repositories' entry
type HelmEntryMetadata struct {
	APIVersion string `yaml:"apiVersion"`
	AppVersion string `yaml:"appVersion"`
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
}

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

	// AppVersion unlike the Version is the actual version of the
	// application. This app version must be present in the
	// https://REPOSITORY/index.yaml
	//
	// If this is defined then chart version will be ignored
	AppVersion string
}

// ApplyHelmChartConfig defines the options that ApplyHelmChart
// can take
type ApplyHelmChartConfig struct {
	// ChartLocation is the remote location of the helm chart
	//
	// Either ChartLocation or URL can be defined, if both of them
	// are defined then URL is given the preferenece
	ChartLocation HelmChartLocation

	// ReleaseName for deploying charts
	ReleaseName string

	// SkipCRDs while installation
	SkipCRDs bool

	// Skip upgrade, if release is already installed
	SkipUpgradeIfInstalled bool

	// URL is the url for charts
	//
	// Either ChartLocation or URL can be defined, if both of them
	// are defined then URL is given the preferenece
	URL string

	// LocalPath is the local path where the routine can find the helm chart
	//
	// If this is provided then both URL and ChartLocation will be completely
	// ignored
	LocalPath string

	// HelmDriver is used to determine the backend
	// information used by helm for managing release
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

	// CreateNamespace creates namespace if it doesn't exist
	//
	// Defaults to false
	CreateNamespace bool

	// OverrideValues are used during installation
	// to override the values present in Values.yaml
	// it is equivalent to --set or --set-file helm flag
	OverrideValues map[string]interface{}

	// Action indicates if the requested action is UNINSTALL, UPGRADE or INSTALL
	//
	// If this is not provided, it performs an INSTALL operation
	Action HelmChartAction

	// Logger that will be used by the client to print the logs
	//
	// If nothing is provided then a dummy logger is used
	Logger func(string, ...interface{})

	// DryRun will skip actual run, useful for testing
	DryRun bool

	// DownloadLocation defines the location where the user wants to download the helm charts
	// If this is not provided, the helm chart is downloaded to the "/tmp" folder
	DownloadLocation string
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
//
//	err = client.ApplyHelmChart(k8s.ApplyHelmChartConfig{
//	        Namespace:       "traefik-mesh",
//	        CreateNamespace: true,
//	        URL:             "https://helm.traefik.io/mesh/traefik-mesh-3.0.6.tgz",
//	})
//
// Install Traefik Mesh using repository:
//
//	err = cl.ApplyHelmChart(k8s.ApplyHelmChartConfig{
//	        ChartLocation: k8s.HelmChartLocation{
//	            Repository: "https://helm.traefik.io/mesh",
//	            Chart:      "traefik-mesh",
//	        },
//	        Namespace:       "traefik-mesh",
//	        CreateNamespace: true,
//	})
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
func (client *Client) ApplyHelmChart(cfg ApplyHelmChartConfig) error {
	setupDefaults(&cfg)

	if err := setupChartVersion(&cfg); err != nil {
		return ErrApplyHelmChart(err)
	}

	localPath, err := getHelmLocalPath(cfg)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	helmChart, err := loader.Load(localPath)
	if err != nil {
		return ErrApplyHelmChart(err)
	}
	if cfg.ReleaseName == "" {
		cfg.ReleaseName = helmChart.Name()
	}
	if err = checkIfInstallable(helmChart); err != nil {
		return ErrApplyHelmChart(err)
	}

	actionConfig, err := createHelmActionConfig(client, cfg)
	if err != nil {
		return ErrApplyHelmChart(err)
	}

	// Before installing a helm chart, check if it already exists in the cluster
	// this is a workaround make the helm chart installation idempotent
	if cfg.Action == INSTALL && !cfg.SkipUpgradeIfInstalled {
		if err := updateActionIfReleaseFound(actionConfig, &cfg, *helmChart); err != nil {
			return ErrApplyHelmChart(err)
		}
	}

	if err := generateAction(actionConfig, cfg)(helmChart); err != nil {
		return ErrApplyHelmChart(err)
	}

	return nil
}

// updateActionIfReleaseFound changes cfg.Action to UPGRADE if the release is found in the cluster
// this is a workaround of making the helm chart installation idempotent
func updateActionIfReleaseFound(actionConfig *action.Configuration, cfg *ApplyHelmChartConfig, c chart.Chart) error {
	releases, err := action.NewList(actionConfig).Run()
	if err != nil {
		return err
	}

	for _, r := range releases {
		if r.Name == cfg.ReleaseName {
			cfg.Action = UPGRADE
			return nil
		}
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
	if cfg.Logger == nil {
		cfg.Logger = func(string, ...interface{}) {} // Dummy logger for helm packages
	}
}

// setupChartVersion takes in the configuration and assigns a chart version
// if an app version is provided. If app version is not provided then it will
// skip any processing
func setupChartVersion(cfg *ApplyHelmChartConfig) error {
	if cfg.ChartLocation.AppVersion != "" {
		var err error
		cfg.ChartLocation.Version, err = HelmConvertAppVersionToChartVersion(
			cfg.ChartLocation.Repository,
			cfg.ChartLocation.Chart,
			cfg.ChartLocation.AppVersion,
		)

		return err
	}

	return nil
}

// getHelmLocalPath takes in the configuration and returns path to helm chart
// on the local file system
//
// If cfg has LocalPath defined then it will skip downloading and assumes that
// the chart exists at the mentioned location
func getHelmLocalPath(cfg ApplyHelmChartConfig) (string, error) {
	if cfg.LocalPath != "" {
		return cfg.LocalPath, nil
	}

	url, err := getHelmChartURL(cfg)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}

	return fetchHelmChart(url, cfg.DownloadLocation)
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
func fetchHelmChart(chartURL, downloadPath string) (string, error) {
	filename := filepath.Base(chartURL)

	// This allows the caller of the function to use the perfered location to download the helm chart, e.g. "~/.meshery/manifests"
	if downloadPath == "" {
		downloadPath = filepath.Join(downloadLocation, filename)
	} else {
		downloadPath = filepath.Join(downloadPath, filename)
	}

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
func createHelmActionConfig(c *Client, cfg ApplyHelmChartConfig) (*action.Configuration, error) {
	// Set the environment variable needed by the Init methods
	os.Setenv("HELM_DRIVER_SQL_CONNECTION_STRING", cfg.SQLConnectionString)

	// KubeConfig setup
	cafile, err := setDataAndReturnFileHandler(c.RestConfig.CAData)
	if err != nil {
		return nil, err
	}
	cafilename := cafile.Name()

	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &c.RestConfig.Host
	kubeConfig.CAFile = &cafilename
	kubeConfig.BearerToken = &c.RestConfig.BearerToken

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kubeConfig, cfg.Namespace, string(cfg.HelmDriver), cfg.Logger); err != nil {
		return nil, ErrApplyHelmChart(err)
	}
	return actionConfig, nil
}

// Populates a file in temp directory with the passed data and returns the data handler
func setDataAndReturnFileHandler(data []byte) (*os.File, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	_, err = f.Write(data)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// generateAction generates an action function using action.Configuration
// and ApplyHelmChartConfig and returns it
//
// The intention is to create a factory function which creates a layer of abstraction
// on top of helm actions making them follow the same interface, hence easing extending
// the number of supported helm actions
func generateAction(actionConfig *action.Configuration, cfg ApplyHelmChartConfig) func(*chart.Chart) error {
	switch cfg.Action {
	case UNINSTALL:
		return func(c *chart.Chart) error {
			act := action.NewUninstall(actionConfig)
			act.DryRun = cfg.DryRun
			if _, err := act.Run(cfg.ReleaseName); err != nil {
				return ErrApplyHelmChart(err)
			}
			return nil
		}
	case UPGRADE:
		return func(c *chart.Chart) error {
			act := action.NewUpgrade(actionConfig)
			act.Namespace = cfg.Namespace
			act.DryRun = cfg.DryRun
			if _, err := act.Run(c.Name(), c, cfg.OverrideValues); err != nil {
				return ErrApplyHelmChart(err)
			}
			return nil
		}
	default:
		return func(c *chart.Chart) error {
			act := action.NewInstall(actionConfig)
			act.ReleaseName = cfg.ReleaseName
			act.CreateNamespace = cfg.CreateNamespace
			act.Namespace = cfg.Namespace
			act.DryRun = cfg.DryRun
			if _, err := act.Run(c, cfg.OverrideValues); err != nil {
				return ErrApplyHelmChart(err)
			}
			return nil
		}
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

// HelmConvertAppVersionToChartVersion takes in the repo, chart and app version and
// returns the corresponding chart version for the same
func HelmConvertAppVersionToChartVersion(repo, chart, appVersion string) (string, error) {
	return HelmAppVersionToChartVersion(repo, chart, normalizeVersion(appVersion))
}

// HelmChartVersionToAppVersion takes in the repo, chart and chart version and
// returns the corresponding app version for the same without normalizing the app version
func HelmChartVersionToAppVersion(repo, chart, chartVersion string) (string, error) {
	helmIndex, err := createHelmIndex(repo)
	if err != nil {
		return "", ErrCreatingHelmIndex(err)
	}

	entryMetadata, exists := helmIndex.Entries.GetEntryWithChartVersion(chart, chartVersion)
	if !exists {
		return "", ErrEntryWithChartVersionNotExists(chart, chartVersion)
	}

	return entryMetadata.AppVersion, nil
}

// HelmAppVersionToChartVersion takes in the repo, chart and app version and
// returns the corresponding chart version for the same without normalizing the app version
func HelmAppVersionToChartVersion(repo, chart, appVersion string) (string, error) {
	helmIndex, err := createHelmIndex(repo)
	if err != nil {
		return "", ErrCreatingHelmIndex(err)
	}

	entryMetadata, exists := helmIndex.Entries.GetEntryWithAppVersion(chart, appVersion)
	if !exists {
		return "", ErrEntryWithAppVersionNotExists(chart, appVersion)
	}

	return entryMetadata.Version, nil
}

// createHelmIndex takes in the repo name and creates a
// helm index for it. Helm index is basically marshaled version of
// index.yaml file present in the remote helm repository
func createHelmIndex(repo string) (*HelmIndex, error) {
	url := fmt.Sprintf("%s/index.yaml", repo)

	// helm repository path will alaways be variable hence,
	// #nosec
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, ErrHelmRepositoryNotFound(repo, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var hi HelmIndex
	dec := yaml.NewDecoder(resp.Body)
	if err := dec.Decode(&hi); err != nil {
		return nil, utils.ErrDecodeYaml(err)
	}

	return &hi, nil
}

// GetEntryWithAppVersion takes in the entry name and the appversion and returns the corresponding
// metadata for the parameters if it exists
func (helmEntries HelmEntries) GetEntryWithAppVersion(entry, appVersion string) (HelmEntryMetadata, bool) {
	hem, ok := helmEntries[entry]
	if !ok {
		return HelmEntryMetadata{}, false
	}

	for _, v := range hem {
		if v.Name == entry && v.AppVersion == appVersion {
			return v, true
		}
	}

	return HelmEntryMetadata{}, false
}

// GetEntryWithAppVersion takes in the entry name and the appversion and returns the corresponding
// metadata for the parameters if it exists
func (helmEntries HelmEntries) GetEntryWithChartVersion(entry, chartVersion string) (HelmEntryMetadata, bool) {
	hem, ok := helmEntries[entry]
	if !ok {
		return HelmEntryMetadata{}, false
	}

	for _, v := range hem {
		if v.Name == entry && v.Version == chartVersion {
			return v, true
		}
	}

	return HelmEntryMetadata{}, false
}

// normalizeVerion takes in a version and adds "v" prefix
// if it isn't already present
func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}

	return fmt.Sprintf("v%s", version)
}
