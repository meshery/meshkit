package controllers

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"net/http"
	"net/url"

	"github.com/meshery/meshery-operator/api/v1alpha1"
	"github.com/meshery/meshkit/utils"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
)

const BrokerPingEndpoint = "/connz"

// connectivityTestTimeout bounds a single broker monitoring-endpoint probe.
const connectivityTestTimeout = 5 * time.Second

type Connections struct {
	Connections []connection `json:"connections"`
}

type connection struct {
	Name string `json:"name"`
}

// parseHostPort splits a "host:port" string into a HostPort. It tolerates an
// empty string and a bare host (returning ok=false) instead of panicking, which
// the previous strings.Split(...)[1] indexing did when an endpoint was empty.
func parseHostPort(hp string) (*utils.HostPort, bool) {
	idx := strings.LastIndex(hp, ":")
	if idx < 0 {
		return nil, false
	}
	port, err := strconv.Atoi(hp[idx+1:])
	if err != nil {
		return nil, false
	}
	return &utils.HostPort{Address: hp[:idx], Port: int32(port)}, true
}

// GetBrokerEndpoint resolves the best reachable broker endpoint from the Broker
// CRD status. Candidates are probed in the order Meshery is commonly deployed
// relative to the cluster:
//  1. internal             — Meshery running in-cluster (ClusterIP)
//  2. external             — Meshery out-of-cluster, broker exposed (NodePort/LB)
//  3. host.docker.internal — Meshery in Docker Desktop reaching the host
//  4. API-server host      — reuse the kube API host with the broker port
//     (e.g. a port-forwarded 127.0.0.1:<port>)
//
// Newer operators deploy NATS via the upstream Helm chart as a ClusterIP-only
// Service, so Status.Endpoint.External is empty. The old implementation
// unconditionally indexed strings.Split(External, ":")[1], panicking on that
// empty value; it also fell back to the internal (unreachable) endpoint even
// when a host.docker.internal / API-host path was reachable. This resolves both.
func GetBrokerEndpoint(kclient *mesherykube.Client, broker *v1alpha1.Broker) string {
	internal := broker.Status.Endpoint.Internal
	external := broker.Status.Endpoint.External

	candidates := []*utils.HostPort{}
	var port int32
	if hp, ok := parseHostPort(internal); ok {
		candidates = append(candidates, hp)
		port = hp.Port
	}
	if hp, ok := parseHostPort(external); ok {
		candidates = append(candidates, hp)
		if port == 0 {
			port = hp.Port
		}
	}
	if port != 0 {
		candidates = append(candidates, &utils.HostPort{Address: "host.docker.internal", Port: port})
		if u, err := url.Parse(kclient.RestConfig.Host); err == nil && u.Hostname() != "" {
			candidates = append(candidates, &utils.HostPort{Address: u.Hostname(), Port: port})
		}
	}

	for _, hp := range candidates {
		if hp != nil && hp.Address != "" && utils.TcpCheck(hp, nil) {
			return hp.String()
		}
	}

	// Nothing verified reachable; return the internal (else external) endpoint as
	// a best-effort default, preserving prior in-cluster behavior.
	if internal != "" {
		return internal
	}
	return external
}

func applyOperatorHelmChart(chartRepo string, client mesherykube.Client, mesheryReleaseVersion string, delete bool, overrides map[string]interface{}) error {
	var (
		act   = mesherykube.INSTALL
		chart = "meshery-operator"
	)
	if delete {
		act = mesherykube.UNINSTALL
	}
	err := client.ApplyHelmChart(mesherykube.ApplyHelmChartConfig{
		Namespace:   "meshery",
		ReleaseName: "meshery-operator",
		ChartLocation: mesherykube.HelmChartLocation{
			Repository: chartRepo,
			Chart:      chart,
			Version:    mesheryReleaseVersion,
		},
		// CreateNamespace doesn't have any effect when the action is UNINSTALL
		CreateNamespace: true,
		Action:          act,
		// Setting override values
		OverrideValues: overrides,

		UpgradeIfInstalled: true,
	})
	if err != nil {
		return err
	}
	return nil
}

func ConnectivityTest(clientName, hostPort string) bool {
	endpoint, err := url.Parse("http://" + hostPort + BrokerPingEndpoint)
	if err != nil {
		return false
	}

	// Bound the probe: the default client has no timeout, so an endpoint that
	// accepts the TCP connection but never responds would hang the caller
	// (and, on the status-poll path, stall the whole status collection).
	client := &http.Client{Timeout: connectivityTestTimeout}
	resp, err := client.Get(endpoint.String())
	if err != nil {
		return false
	}
	// Always close the body — otherwise every probe leaks a connection/FD, which
	// accumulates quickly on the periodic status poll.
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var natsResponse Connections
	err = json.Unmarshal(body, &natsResponse)
	if err != nil {
		return false
	}

	for _, client := range natsResponse.Connections {
		if client.Name == clientName {
			return true
		}
	}
	return false
}
