package cloudauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/logger"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func testCertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cluster-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func b64PEM(pemBytes []byte) string {
	return base64.StdEncoding.EncodeToString(pemBytes)
}

func urlHostIP(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil {
		t.Fatalf("host %q is not an IP", parsed.Hostname())
	}
	return ip.String()
}

func clusterFor(ip string, caPEM []byte) gkeCluster {
	var cluster gkeCluster
	cluster.Name = "demo"
	cluster.Location = "us-central1"
	cluster.Endpoint = ip
	cluster.MasterAuth.ClusterCaCertificate = b64PEM(caPEM)
	return cluster
}

// containerAPI is a fake GKE Container API. It records every request and
// never forwards them.
type containerAPI struct {
	*httptest.Server
	mu       sync.Mutex
	gets     []string
	lists    []string
	auth     []string
	status   int
	clusters []servedCluster
	// projectStatus answers every call for a project with that status.
	// getStatus and listStatus answer every clusters.get or clusters.list
	// call with that status.
	projectStatus map[string]int
	getStatus     int
	listStatus    int
	// missingZones is reported on every clusters.list response.
	missingZones []string
	// entered and cancelled are used by the blocking handler.
	entered   chan struct{}
	cancelled chan struct{}
	block     bool
}

type servedCluster struct {
	Project  string
	Location string
	Name     string
	Cluster  gkeCluster
}

func startContainerAPI(t *testing.T, clusters []servedCluster) *containerAPI {
	t.Helper()
	api := &containerAPI{clusters: clusters}
	api.Server = httptest.NewServer(api)
	t.Cleanup(api.Close)
	return api
}

func (a *containerAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.auth = append(a.auth, r.Header.Get("Authorization"))
	block := a.block
	a.mu.Unlock()
	if block {
		a.entered <- struct{}{}
		<-r.Context().Done()
		a.cancelled <- struct{}{}
		return
	}
	isList := strings.Contains(r.URL.Path, "/locations/-/clusters")
	status := a.status
	if override, ok := a.projectStatus[projectFromListPath(r.URL.Path)]; ok {
		status = override
	}
	if !isList && a.getStatus != 0 {
		status = a.getStatus
	}
	if isList && a.listStatus != 0 {
		status = a.listStatus
	}
	if status != 0 {
		a.mu.Lock()
		if isList {
			a.lists = append(a.lists, projectFromListPath(r.URL.Path))
		} else {
			a.gets = append(a.gets, r.URL.Path)
		}
		a.mu.Unlock()
		w.WriteHeader(status)
		return
	}
	if strings.Contains(r.URL.Path, "/locations/-/clusters") {
		project := projectFromListPath(r.URL.Path)
		a.mu.Lock()
		a.lists = append(a.lists, project)
		var clusters []gkeCluster
		for _, cluster := range a.clusters {
			if cluster.Project == project {
				clusters = append(clusters, cluster.Cluster)
			}
		}
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gkeListResponse{Clusters: clusters, MissingZones: a.missingZones})
		return
	}
	a.mu.Lock()
	a.gets = append(a.gets, r.URL.Path)
	var found *gkeCluster
	for i := range a.clusters {
		cluster := &a.clusters[i]
		if r.URL.Path == clusterGetPath(gkeNamedCluster{Project: cluster.Project, Location: cluster.Location, Cluster: cluster.Name}) {
			found = &cluster.Cluster
			break
		}
	}
	a.mu.Unlock()
	if found == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(found)
}

func projectFromListPath(path string) string {
	const prefix = "/v1/projects/"
	rest := strings.TrimPrefix(path, prefix)
	project, _, _ := strings.Cut(rest, "/")
	decoded, err := url.PathUnescape(project)
	if err != nil {
		return project
	}
	return decoded
}

func (a *containerAPI) getPaths() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.gets...)
}

func (a *containerAPI) listProjects() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.lists...)
}

func (a *containerAPI) authorizations() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.auth...)
}

func gkeIPConfig(host string, ca []byte, exec *clientcmdapi.ExecConfig) *rest.Config {
	return &rest.Config{
		Host:            host,
		ExecProvider:    exec,
		TLSClientConfig: rest.TLSClientConfig{CAData: ca},
	}
}

func TestParseGKEClusterName(t *testing.T) {
	tests := []struct {
		input    string
		project  string
		location string
		cluster  string
		ok       bool
	}{
		{input: "gke_my-project_us-central1_demo", project: "my-project", location: "us-central1", cluster: "demo", ok: true},
		{input: "gke_my-project_us-central1-a_demo", project: "my-project", location: "us-central1-a", cluster: "demo", ok: true},
		{input: "gke_my-project_europe-west1_my-cluster", project: "my-project", location: "europe-west1", cluster: "my-cluster", ok: true},
		{input: "gke_my-project_europe-west10_demo", project: "my-project", location: "europe-west10", cluster: "demo", ok: true},
		{input: "gke_my-project_europe-west12-b_demo", project: "my-project", location: "europe-west12-b", cluster: "demo", ok: true},
		{input: "gke_my-project_northamerica-northeast1_my-cluster-1", project: "my-project", location: "northamerica-northeast1", cluster: "my-cluster-1", ok: true},
		{input: "gke_my-project_us-east1_a", project: "my-project", location: "us-east1", cluster: "a", ok: true},
		{input: "gke_my-project_asia-southeast2_prod", project: "my-project", location: "asia-southeast2", cluster: "prod", ok: true},
		{input: "gke_abcdef_us-west1-c_demo", project: "abcdef", location: "us-west1-c", cluster: "demo", ok: true},
		{input: "gke_ab_us-central1_demo"},
		{input: "gke_proj_us-central1_demo"},
		{input: "gke_my-project_us-central1"},
		{input: "gke_my-project_not-a-location_demo"},
		{input: "gke_MY-PROJECT_us-central1_demo"},
		{input: "gke_my-project_US-CENTRAL1_demo"},
		{input: "gke_my-project_us-central1-ab_demo"},
		{input: "gke_my-project_europe-west10-ab_demo"},
		{input: "gke_my-project_us-central1_my_cluster"},
		{input: "gke_my-project_us-central1_"},
		{input: "not-gke_my-project_us-central1_demo"},
		{input: "gke_my-project-_us-central1_demo"},
		{input: ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseGKEClusterName(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseGKEClusterName(%q) ok = %v, want %v (%+v)", tt.input, ok, tt.ok, got)
			}
			if !tt.ok {
				return
			}
			if got.Project != tt.project || got.Location != tt.location || got.Cluster != tt.cluster {
				t.Fatalf("parseGKEClusterName(%q) = %+v, want %s/%s/%s", tt.input, got, tt.project, tt.location, tt.cluster)
			}
		})
	}
}

func TestCertificateSetsAcceptOnlyPEMAndAPIBase64OfPEM(t *testing.T) {
	pemBytes := testCertPEM(t)
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("pem.Decode returned nil")
	}
	kubeconfig, err := pemCertificateSet(pemBytes)
	if err != nil {
		t.Fatalf("pemCertificateSet(PEM) error = %v", err)
	}
	api, err := apiCertificateSet(b64PEM(pemBytes))
	if err != nil {
		t.Fatalf("apiCertificateSet(base64 of PEM) error = %v", err)
	}
	if !derSetsEqual(kubeconfig, api) {
		t.Fatal("the kubeconfig PEM and the API base64 of the same PEM did not compare equal")
	}

	for name, form := range map[string][]byte{
		"DER":           block.Bytes,
		"base64 of PEM": []byte(b64PEM(pemBytes)),
		"base64 of DER": []byte(base64.StdEncoding.EncodeToString(block.Bytes)),
	} {
		if _, err := pemCertificateSet(form); err == nil {
			t.Fatalf("pemCertificateSet accepted a kubeconfig CA given as %s", name)
		}
	}
	padded := append([]byte(nil), pemBytes...)
	for len(padded)%3 == 0 {
		padded = append(padded, '\n')
	}
	for name, form := range map[string]string{
		"PEM":                    string(pemBytes),
		"base64 of DER":          base64.StdEncoding.EncodeToString(block.Bytes),
		"unpadded base64 of PEM": base64.RawStdEncoding.EncodeToString(padded),
	} {
		if _, err := apiCertificateSet(form); err == nil {
			t.Fatalf("apiCertificateSet accepted clusterCaCertificate given as %s", name)
		}
	}

	other, err := pemCertificateSet(testCertPEM(t))
	if err != nil {
		t.Fatalf("pemCertificateSet(other) error = %v", err)
	}
	if derSetsEqual(kubeconfig, other) {
		t.Fatal("two different certificates compared equal")
	}
}

func TestParseEndpointIP(t *testing.T) {
	want := net.ParseIP("2600:1900:4000::1")
	for _, raw := range []string{"2600:1900:4000::1", "[2600:1900:4000::1]", "[2600:1900:4000::1]:443", "2600:1900:4000:0:0:0:0:1"} {
		if got := parseEndpointIP(raw); !want.Equal(got) {
			t.Fatalf("parseEndpointIP(%q) = %v, want %v", raw, got, want)
		}
	}
	if got := parseEndpointIP("34.118.10.20:443"); !got.Equal(net.ParseIP("34.118.10.20")) {
		t.Fatalf("parseEndpointIP(v4 with port) = %v", got)
	}
	if parseEndpointIP("demo.gke.goog") != nil || parseEndpointIP("") != nil {
		t.Fatal("parseEndpointIP accepted a non-IP")
	}
}

func TestClusterMatchRequiresIPAndCAOnEveryEndpointField(t *testing.T) {
	ca := testCertPEM(t)
	other := testCertPEM(t)
	wantIP := net.ParseIP("10.0.0.5")
	wantCA, err := pemCertificateSet(ca)
	if err != nil {
		t.Fatal(err)
	}
	fields := []func(*gkeCluster){
		func(c *gkeCluster) { c.Endpoint = "10.0.0.5" },
		func(c *gkeCluster) { c.PrivateClusterConfig.PublicEndpoint = "10.0.0.5" },
		func(c *gkeCluster) { c.PrivateClusterConfig.PrivateEndpoint = "10.0.0.5" },
		func(c *gkeCluster) { c.ControlPlaneEndpointsConfig.IPEndpointsConfig.PublicEndpoint = "10.0.0.5" },
		func(c *gkeCluster) {
			c.ControlPlaneEndpointsConfig.IPEndpointsConfig.PrivateEndpoint = "[10.0.0.5]:443"
		},
	}
	for i, set := range fields {
		cluster := gkeCluster{}
		cluster.MasterAuth.ClusterCaCertificate = b64PEM(ca)
		set(&cluster)
		if !clusterMatches(cluster, wantIP, wantCA) {
			t.Fatalf("field %d did not match", i)
		}
		cluster.MasterAuth.ClusterCaCertificate = b64PEM(other)
		if clusterMatches(cluster, wantIP, wantCA) {
			t.Fatalf("field %d matched a different CA", i)
		}
	}
	cluster := clusterFor("10.0.0.6", ca)
	if clusterMatches(cluster, wantIP, wantCA) {
		t.Fatal("CA match with a different IP was accepted")
	}
}

func TestGKEIPCacheKeySeparatesEndpoints(t *testing.T) {
	base := Spec{Provider: ProviderGKE, gkeIP: &gkeIPEndpoint{Host: "1.2.3.4", CA: []byte("ca")}}
	otherIP := Spec{Provider: ProviderGKE, gkeIP: &gkeIPEndpoint{Host: "1.2.3.5", CA: []byte("ca")}}
	otherCA := Spec{Provider: ProviderGKE, gkeIP: &gkeIPEndpoint{Host: "1.2.3.4", CA: []byte("other")}}
	dns := Spec{Provider: ProviderGKE}
	if base.cacheKey() == otherIP.cacheKey() || base.cacheKey() == otherCA.cacheKey() || base.cacheKey() == dns.cacheKey() {
		t.Fatal("GKE IP cache keys collided")
	}
	same := Spec{Provider: ProviderGKE, gkeIP: &gkeIPEndpoint{Host: "1.2.3.4", CA: []byte("ca"), ContextName: "other"}}
	if same.cacheKey() != base.cacheKey() {
		t.Fatal("context name changed the cache key")
	}
}

func TestGKEGetMatchInstallsTokenAndWarns(t *testing.T) {
	server := newAuthorizationRecorder(t, "adc-token")
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	ip := urlHostIP(t, server.URL)
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(ip, ca),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	log, logs := bufferLogger(t)
	var calls atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", &calls)),
		withLookPath(notOnPath),
		withLogger(log),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig(server.URL, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_demo", "ignored"); err != nil {
		t.Fatalf("ApplyWithClusterHint() error = %v", err)
	}
	if config.ExecProvider != nil || config.WrapTransport == nil {
		t.Fatal("Apply() did not install the token source")
	}
	if calls.Load() != 1 {
		t.Fatalf("factory calls = %d, want 1", calls.Load())
	}
	if got := api.listProjects(); len(got) != 0 {
		t.Fatalf("list calls = %v, want none after a get match", got)
	}
	if got := api.getPaths(); len(got) != 1 || !strings.Contains(got[0], "/projects/my-project/locations/us-central1/clusters/demo") {
		t.Fatalf("get paths = %v", got)
	}
	if authz := api.authorizations(); len(authz) != 1 || authz[0] != "Bearer adc-token" {
		t.Fatalf("container API Authorization = %v", authz)
	}
	if server.hits.Load() != 0 {
		t.Fatal("the kubeconfig server was contacted before verification finished")
	}
	if got := getStatus(t, config); got != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", got)
	}
	if !strings.Contains(logs.String(), ErrInProcessGKEIPMintingCode) ||
		!strings.Contains(logs.String(), "In-process GKE token minting is active for IP endpoint "+ip) ||
		!strings.Contains(logs.String(), "cluster my-project/us-central1/demo") {
		t.Fatalf("logs = %q, want one %s warning", logs.String(), ErrInProcessGKEIPMintingCode)
	}

	// A second Apply reuses the setup: no second lookup and no second warning.
	again := gkeIPConfig(server.URL, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(again, "gke_my-project_us-central1_demo", ""); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if again.ExecProvider != nil {
		t.Fatal("second Apply() did not install the cached source")
	}
	if calls.Load() != 1 || len(api.getPaths()) != 1 {
		t.Fatalf("factory calls = %d, get calls = %d, want both still 1", calls.Load(), len(api.getPaths()))
	}
	if lines := strings.Split(strings.TrimSpace(logs.String()), "\n"); len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1 per setup: %s", len(lines), logs.String())
	}
}

func TestGKEListMatchWithoutNameHint(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.20"
	api := startContainerAPI(t, []servedCluster{{
		Project: "env-project", Location: "europe-west1", Name: "prod",
		Cluster: func() gkeCluster {
			cluster := clusterFor(ip, ca)
			cluster.Name = "prod"
			cluster.Location = "europe-west1"
			cluster.Endpoint = ""
			cluster.PrivateClusterConfig.PrivateEndpoint = ip
			return cluster
		}(),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "env-project")
	log, logs := bufferLogger(t)
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withLogger(log),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("list match did not install the token source")
	}
	if len(api.getPaths()) != 0 {
		t.Fatalf("get paths = %v, want none without a gke_ name", api.getPaths())
	}
	if got := api.listProjects(); len(got) != 1 || got[0] != "env-project" {
		t.Fatalf("list projects = %v", got)
	}
	if !strings.Contains(logs.String(), "cluster env-project/europe-west1/prod") {
		t.Fatalf("logs = %q, want the listed cluster", logs.String())
	}
}

func TestGKEListMatchAfterWrongName(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.21"
	cluster := clusterFor(ip, ca)
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo", Cluster: cluster,
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_missing", "also-wrong"); err != nil {
		t.Fatalf("ApplyWithClusterHint() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("list fallback did not install the token source")
	}
	if len(api.getPaths()) != 1 || len(api.listProjects()) != 1 {
		t.Fatalf("gets = %v lists = %v, want one get and one list", api.getPaths(), api.listProjects())
	}
}

func TestGKEListUsesADCProject(t *testing.T) {
	adcProjectID = defaultADCProjectID
	t.Cleanup(func() { adcProjectID = func(context.Context) string { return "" } })
	useADCFile(t, fakeServiceAccountKey(t, map[string]string{"project_id": "adc-project1"}))

	ca := testCertPEM(t)
	const ip = "34.118.10.22"
	api := startContainerAPI(t, []servedCluster{{
		Project: "adc-project1", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(ip, ca),
	}})
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := api.listProjects(); len(got) != 1 || got[0] != "adc-project1" {
		t.Fatalf("list projects = %v, want the ADC project once", got)
	}

	// The same project from the environment is not listed twice.
	t.Setenv("GOOGLE_CLOUD_PROJECT", "adc-project1")
	t.Setenv("GOOGLE_PROJECT", "adc-project1")
	other := gkeIPConfig("https://34.118.10.23", ca, gkeExecConfig())
	_ = auth.Apply(other)
	adcLists := 0
	for _, project := range api.listProjects() {
		if project == "adc-project1" {
			adcLists++
		}
	}
	if adcLists != 2 {
		t.Fatalf("adc-project1 lists = %d, want 2 (one per setup, not one per duplicate source): %v", adcLists, api.listProjects())
	}
}

func TestGKEIPMismatchIsRefused(t *testing.T) {
	realCA := testCertPEM(t)
	attackerCA := testCertPEM(t)
	const realIP = "34.118.10.30"
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "real",
		Cluster: clusterFor(realIP, realCA),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")

	t.Run("attacker IP and CA, including a gke_ name for the real cluster", func(t *testing.T) {
		attacker := newAuthorizationRecorder(t, "adc-token")
		var calls atomic.Int32
		auth := NewAuthenticator(
			WithTokenSourceFactory(staticFactory("adc-token", &calls)),
			withLookPath(notOnPath),
			withContainerAPIBase(api.URL),
		)
		config := gkeIPConfig(attacker.URL, attackerCA, gkeExecConfig())
		err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_real", "gke_my-project_us-central1_real")
		if got := errors.GetCode(err); got != ErrUntrustedClusterEndpointCode {
			t.Fatalf("Apply() error = %v, want code %q", err, ErrUntrustedClusterEndpointCode)
		}
		if !strings.Contains(err.Error(), "no cluster matched") {
			t.Fatalf("error = %v, want the no-match cause", err)
		}
		if config.ExecProvider == nil || config.WrapTransport != nil || config.BearerToken != "" {
			t.Fatal("Apply() installed a token for the attacker endpoint")
		}
		if attacker.hits.Load() != 0 {
			t.Fatalf("attacker received %d requests during Apply", attacker.hits.Load())
		}
		config.ExecProvider = nil
		_ = getStatus(t, config)
		if got := attacker.seen.Load(); got != "" {
			t.Fatalf("attacker Authorization = %q", got)
		}
		if calls.Load() != 1 {
			t.Fatalf("factory calls = %d, want 1 (the Google token is built, then not sent to the attacker)", calls.Load())
		}
		for _, header := range api.authorizations() {
			if header != "Bearer adc-token" {
				t.Fatalf("container API Authorization = %q", header)
			}
		}
	})

	t.Run("IP matches and CA differs", func(t *testing.T) {
		auth := NewAuthenticator(
			WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
			withLookPath(notOnPath),
			withContainerAPIBase(api.URL),
		)
		before := len(api.authorizations())
		config := gkeIPConfig("https://"+realIP, attackerCA, gkeExecConfig())
		err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_real", "")
		if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "no cluster matched") {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.WrapTransport != nil {
			t.Fatal("token installed despite a CA mismatch")
		}
		if len(api.authorizations()) == before {
			t.Fatal("the Container API was not consulted")
		}
	})

	t.Run("CA matches and IP differs", func(t *testing.T) {
		auth := NewAuthenticator(
			WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
			withLookPath(notOnPath),
			withContainerAPIBase(api.URL),
		)
		config := gkeIPConfig("https://203.0.113.10", realCA, gkeExecConfig())
		err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_real", "")
		if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "no cluster matched") {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.WrapTransport != nil {
			t.Fatal("token installed despite an IP mismatch")
		}
	})
}

func TestGKELookupFailureUsesRefusedEndpointPath(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.40"
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	tests := []struct {
		name   string
		reason string
		api    func(t *testing.T) (base string, hits func() int)
	}{
		{
			name:   "permission denied",
			reason: "permission denied",
			api: func(t *testing.T) (string, func() int) {
				api := startContainerAPI(t, nil)
				api.status = http.StatusForbidden
				return api.URL, func() int { return len(api.authorizations()) }
			},
		},
		{
			name:   "no matching cluster",
			reason: "no cluster matched",
			api: func(t *testing.T) (string, func() int) {
				cluster := clusterFor("198.51.100.10", ca)
				api := startContainerAPI(t, []servedCluster{{
					Project: "my-project", Location: "us-central1", Name: "other", Cluster: cluster,
				}})
				return api.URL, func() int { return len(api.authorizations()) }
			},
		},
		{
			name:   "server error",
			reason: "HTTP 503",
			api: func(t *testing.T) (string, func() int) {
				api := startContainerAPI(t, nil)
				api.status = http.StatusServiceUnavailable
				return api.URL, func() int { return len(api.authorizations()) }
			},
		},
		{
			name:   "connection refused",
			reason: "GKE API lookup failed",
			api: func(t *testing.T) (string, func() int) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				addr := listener.Addr().String()
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
				return "http://" + addr, func() int { return 0 }
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, _ := tt.api(t)
			newConfig := func() *rest.Config {
				return gkeIPConfig("https://"+ip, ca, gkeExecConfig())
			}
			auth := NewAuthenticator(
				WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
				withLookPath(notOnPath),
				withContainerAPIBase(base),
			)
			config := newConfig()
			exec := config.ExecProvider
			err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_demo", "")
			if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("Apply() error = %v, want code %s containing %q", err, ErrUntrustedClusterEndpointCode, tt.reason)
			}
			if config.ExecProvider != exec || config.WrapTransport != nil {
				t.Fatal("Apply() modified the config of a refused endpoint")
			}

			log, logs := bufferLogger(t)
			auth = NewAuthenticator(
				WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
				withLookPath(onPath),
				withLogger(log),
				withContainerAPIBase(base),
			)
			config = newConfig()
			exec = config.ExecProvider
			if err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_demo", ""); err != nil {
				t.Fatalf("Apply() with the plugin on PATH error = %v", err)
			}
			if config.ExecProvider != exec || config.WrapTransport != nil {
				t.Fatal("Apply() installed in-process minting after a failed lookup")
			}
			if !strings.Contains(logs.String(), ErrUntrustedClusterEndpointFallbackCode) || !strings.Contains(logs.String(), tt.reason) {
				t.Fatalf("logs = %q, want %s and %q", logs.String(), ErrUntrustedClusterEndpointFallbackCode, tt.reason)
			}
		})
	}
}

func TestGKELookupReportsEachUnansweredProject(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.45"
	unrelated := servedCluster{
		Project: "adc-project", Location: "us-central1", Name: "unrelated",
		Cluster: clusterFor("198.51.100.20", ca),
	}
	tests := []struct {
		name          string
		projectStatus map[string]int
		getStatus     int
		listStatus    int
		missingZones  []string
		hinted        *servedCluster
		want          []string
		notWant       []string
		lists         []string
	}{
		{
			name:          "a denial on the hinted project is not hidden by another project's answer",
			projectStatus: map[string]int{"cluster-project": http.StatusForbidden},
			want: []string{
				"permission denied for project cluster-project",
				"container.clusters.get or container.clusters.list",
				"no cluster matched",
			},
			lists: []string{"cluster-project", "adc-project"},
		},
		{
			name:          "an error on the hinted project is not hidden by another project's answer",
			projectStatus: map[string]int{"cluster-project": http.StatusServiceUnavailable},
			want:          []string{"HTTP 503 for project cluster-project", "no cluster matched"},
			notWant:       []string{"permission denied"},
			lists:         []string{"cluster-project", "adc-project"},
		},
		{
			name:      "a get denial is answered by a list of the same project",
			getStatus: http.StatusForbidden,
			want:      []string{"no cluster matched"},
			notWant:   []string{"permission denied"},
			lists:     []string{"cluster-project", "adc-project"},
		},
		{
			name: "a get answered for another cluster does not hide the project's list denial",
			hinted: &servedCluster{
				Project: "cluster-project", Location: "us-central1", Name: "demo",
				Cluster: clusterFor("198.51.100.21", ca),
			},
			listStatus: http.StatusForbidden,
			want: []string{
				"no cluster matched",
				"permission denied for project cluster-project",
			},
			lists: []string{"cluster-project", "adc-project"},
		},
		{
			name:         "a list with missing zones is a failed lookup, not a no-match",
			missingZones: []string{"us-central1-a", "us-central1-b"},
			want: []string{
				"could not list zones us-central1-a, us-central1-b for project cluster-project",
				"could not list zones us-central1-a, us-central1-b for project adc-project",
			},
			notWant: []string{"no cluster matched"},
			lists:   []string{"cluster-project", "adc-project"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOOGLE_CLOUD_PROJECT", "cluster-project")
			t.Setenv("GOOGLE_PROJECT", "adc-project")
			served := []servedCluster{unrelated}
			if tt.hinted != nil {
				served = append(served, *tt.hinted)
			}
			api := &containerAPI{
				clusters:      served,
				projectStatus: tt.projectStatus,
				getStatus:     tt.getStatus,
				listStatus:    tt.listStatus,
				missingZones:  tt.missingZones,
			}
			api.Server = httptest.NewServer(api)
			t.Cleanup(api.Close)
			auth := NewAuthenticator(
				WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
				withLookPath(notOnPath),
				withContainerAPIBase(api.URL),
			)
			config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
			err := auth.ApplyWithClusterHint(config, "gke_cluster-project_us-central1_demo", "")
			if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
				t.Fatalf("Apply() error = %v, want code %s", err, ErrUntrustedClusterEndpointCode)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Apply() error = %v, want it to contain %q", err, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(err.Error(), notWant) {
					t.Fatalf("Apply() error = %v, want it not to contain %q", err, notWant)
				}
			}
			if got := api.listProjects(); strings.Join(got, ",") != strings.Join(tt.lists, ",") {
				t.Fatalf("list projects = %v, want %v", got, tt.lists)
			}
			if config.WrapTransport != nil {
				t.Fatal("Apply() installed a token after a failed lookup")
			}
		})
	}
}

func TestGKEPartialListStillAcceptsAMatch(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.46"
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	api := &containerAPI{
		clusters:     []servedCluster{{Project: "my-project", Location: "us-central1", Name: "demo", Cluster: clusterFor(ip, ca)}},
		missingZones: []string{"us-central1-a"},
	}
	api.Server = httptest.NewServer(api)
	t.Cleanup(api.Close)
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider != nil || config.WrapTransport == nil {
		t.Fatal("a match in a partial list did not install the token source")
	}
}

func TestGKEUntrustedHintProjectIsNeverQueried(t *testing.T) {
	attacker := newAuthorizationRecorder(t, "adc-token")
	attackerCA := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: attacker.Certificate().Raw})
	attackerIP := urlHostIP(t, attacker.URL)
	const hint = "gke_attacker-project_us-central1_evil"

	tests := []struct {
		name    string
		trusted string
		want    string
		lists   []string
	}{
		{name: "a trusted project is searched instead", trusted: "meshery-project", want: "no cluster matched", lists: []string{"meshery-project"}},
		{name: "no trusted project makes no API call", want: "no trusted project to search"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.trusted != "" {
				t.Setenv("GOOGLE_CLOUD_PROJECT", tt.trusted)
			}
			// The attacker's project really has a cluster with the kubeconfig
			// IP and CA, so a single request for it would install a token.
			api := startContainerAPI(t, []servedCluster{{
				Project: "attacker-project", Location: "us-central1", Name: "evil",
				Cluster: clusterFor(attackerIP, attackerCA),
			}})
			auth := NewAuthenticator(
				WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
				withLookPath(notOnPath),
				withContainerAPIBase(api.URL),
			)
			config := gkeIPConfig(attacker.URL, attackerCA, gkeExecConfig())
			err := auth.ApplyWithClusterHint(config, hint, hint)
			if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
				t.Fatalf("Apply() error = %v, want code %s", err, ErrUntrustedClusterEndpointCode)
			}
			if !strings.Contains(err.Error(), tt.want) ||
				!strings.Contains(err.Error(), "names project attacker-project, which is not the ADC project") {
				t.Fatalf("Apply() error = %v, want %q and the untrusted hint project", err, tt.want)
			}
			for _, path := range api.getPaths() {
				if strings.Contains(path, "attacker-project") {
					t.Fatalf("clusters.get was called for the untrusted project: %s", path)
				}
			}
			if got := api.listProjects(); strings.Join(got, ",") != strings.Join(tt.lists, ",") {
				t.Fatalf("list projects = %v, want %v", got, tt.lists)
			}
			if tt.trusted == "" && len(api.authorizations()) != 0 {
				t.Fatalf("container API requests = %d, want none without a trusted project", len(api.authorizations()))
			}
			if config.WrapTransport != nil || config.ExecProvider == nil {
				t.Fatal("Apply() installed a token for the attacker endpoint")
			}
			if attacker.hits.Load() != 0 {
				t.Fatalf("attacker received %d requests", attacker.hits.Load())
			}
		})
	}
}

func TestGKELaterTrustedHintIsUsedForTheSameEndpoint(t *testing.T) {
	server := newAuthorizationRecorder(t, "adc-token")
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	ip := urlHostIP(t, server.URL)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	// The identity may get the cluster but not list the project, so only a
	// gke_ name can verify this endpoint.
	api := &containerAPI{
		clusters:   []servedCluster{{Project: "my-project", Location: "us-central1", Name: "demo", Cluster: clusterFor(ip, ca)}},
		listStatus: http.StatusForbidden,
	}
	api.Server = httptest.NewServer(api)
	t.Cleanup(api.Close)
	clock := newFakeClock()
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	auth.clock = clock

	hintless := gkeIPConfig(server.URL, ca, gkeExecConfig())
	err := auth.ApplyWithClusterHint(hintless, "prod", "prod")
	if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "permission denied for project my-project") {
		t.Fatalf("hintless Apply() error = %v, want %s with the list denial", err, ErrUntrustedClusterEndpointCode)
	}
	if got := api.getPaths(); len(got) != 0 {
		t.Fatalf("get paths = %v, want none for a kubeconfig without a gke_ name", got)
	}

	hinted := gkeIPConfig(server.URL, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(hinted, "gke_my-project_us-central1_demo", ""); errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("hinted Apply() within the refusal window error = %v, want the cached refusal", err)
	}

	clock.Advance(gkeRejectionRetryInterval)
	again := gkeIPConfig(server.URL, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(again, "prod", "prod"); err != nil {
		t.Fatalf("Apply() after the window error = %v, want the earlier gke_ name to verify the endpoint", err)
	}
	if again.ExecProvider != nil || again.WrapTransport == nil {
		t.Fatal("Apply() after the window did not install the token source")
	}
	if got := api.getPaths(); len(got) != 1 || !strings.Contains(got[0], "/projects/my-project/locations/us-central1/clusters/demo") {
		t.Fatalf("get paths = %v, want one get for the hinted cluster", got)
	}
	if server.hits.Load() != 0 {
		t.Fatal("the kubeconfig server was contacted during Apply")
	}
}

// getOnlyAuthenticator returns an Authenticator on a fake clock whose
// Container API serves the cluster at ip and ca to clusters.get but answers
// every clusters.list with 403, so only a gke_ name can verify the endpoint.
func getOnlyAuthenticator(t *testing.T, ip string, ca []byte) (*Authenticator, *containerAPI, *fakeClock) {
	t.Helper()
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	api := &containerAPI{
		clusters:   []servedCluster{{Project: "my-project", Location: "us-central1", Name: "demo", Cluster: clusterFor(ip, ca)}},
		listStatus: http.StatusForbidden,
	}
	api.Server = httptest.NewServer(api)
	t.Cleanup(api.Close)
	clock := newFakeClock()
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	auth.clock = clock
	return auth, api, clock
}

func TestGKEWrongEarlierHintDoesNotHideALaterOne(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.48"
	auth, api, clock := getOnlyAuthenticator(t, ip, ca)
	apply := func(hint string) error {
		return auth.ApplyWithClusterHint(gkeIPConfig("https://"+ip, ca, gkeExecConfig()), hint, "")
	}

	if err := apply("gke_my-project_us-central1_wrong"); errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("Apply(wrong name) error = %v, want %s", err, ErrUntrustedClusterEndpointCode)
	}
	if err := apply("gke_my-project_us-central1_demo"); errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("Apply(right name) within the refusal window error = %v, want the cached refusal", err)
	}

	clock.Advance(gkeRejectionRetryInterval)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.ApplyWithClusterHint(config, "gke_my-project_us-central1_wrong", ""); err != nil {
		t.Fatalf("Apply() after the window error = %v, want the later right name to verify the endpoint", err)
	}
	if config.ExecProvider != nil || config.WrapTransport == nil {
		t.Fatal("Apply() after the window did not install the token source")
	}
	gets := api.getPaths()
	if len(gets) != 3 || !strings.HasSuffix(gets[0], "/clusters/wrong") ||
		!strings.HasSuffix(gets[1], "/clusters/wrong") || !strings.HasSuffix(gets[2], "/clusters/demo") {
		t.Fatalf("get paths = %v, want wrong in the first setup, then wrong (seen again, so newest) and demo", gets)
	}
}

func TestGKEHintsAreCappedPerEndpoint(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.49"
	auth, api, clock := getOnlyAuthenticator(t, ip, ca)
	names := []string{"wrong1", "wrong2", "wrong3", "wrong4", "wrong1", "wrong5", "wrong6"}
	for _, name := range names {
		err := auth.ApplyWithClusterHint(gkeIPConfig("https://"+ip, ca, gkeExecConfig()), "gke_my-project_us-central1_"+name, "")
		if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
			t.Fatalf("Apply(%s) error = %v, want %s", name, err, ErrUntrustedClusterEndpointCode)
		}
	}
	before := len(api.getPaths())

	clock.Advance(gkeRejectionRetryInterval)
	err := auth.ApplyWithClusterHint(gkeIPConfig("https://"+ip, ca, gkeExecConfig()), "prod", "")
	if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("Apply() after the window error = %v, want %s", err, ErrUntrustedClusterEndpointCode)
	}
	gets := api.getPaths()[before:]
	var got []string
	for _, path := range gets {
		got = append(got, path[strings.LastIndex(path, "/")+1:])
	}
	if want := []string{"wrong6", "wrong5", "wrong1", "wrong4"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("get calls in one setup = %v, want the %d most recently seen names, newest first, with wrong1 kept because it was seen again: %v", got, gkeMaxHints, want)
	}
}

func TestGKERefusalIsReusedForFiveMinutes(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.47"
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	for _, on := range []bool{false, true} {
		look, name := notOnPath, "plugin not on PATH"
		if on {
			look, name = onPath, "plugin on PATH"
		}
		t.Run(name, func(t *testing.T) {
			api := startContainerAPI(t, []servedCluster{{
				Project: "my-project", Location: "us-central1", Name: "other",
				Cluster: clusterFor("198.51.100.30", ca),
			}})
			clock := newFakeClock()
			var calls atomic.Int32
			auth := NewAuthenticator(
				WithTokenSourceFactory(staticFactory("adc-token", &calls)),
				withLookPath(look),
				withContainerAPIBase(api.URL),
				withLogger(discardLogger(t)),
			)
			auth.clock = clock
			apply := func(when string) {
				t.Helper()
				config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
				err := auth.Apply(config)
				if on && err != nil {
					t.Fatalf("%s: Apply() with the plugin on PATH error = %v", when, err)
				}
				if !on && (errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "no cluster matched")) {
					t.Fatalf("%s: Apply() error = %v, want %s with the lookup cause", when, err, ErrUntrustedClusterEndpointCode)
				}
				if config.ExecProvider == nil || config.WrapTransport != nil {
					t.Fatalf("%s: Apply() installed a token for a refused endpoint", when)
				}
			}

			apply("first Apply")
			requests := len(api.authorizations())
			if calls.Load() != 1 || requests == 0 {
				t.Fatalf("first Apply: factory calls = %d, API requests = %d, want 1 and some", calls.Load(), requests)
			}

			clock.Advance(gkeRejectionRetryInterval - time.Second)
			apply("Apply within the window")
			if calls.Load() != 1 || len(api.authorizations()) != requests {
				t.Fatalf("Apply within the window: factory calls = %d, API requests = %d, want 1 and %d", calls.Load(), len(api.authorizations()), requests)
			}

			clock.Advance(time.Second)
			apply("Apply after the window")
			if calls.Load() != 2 || len(api.authorizations()) <= requests {
				t.Fatalf("Apply after the window: factory calls = %d, API requests = %d, want 2 and more than %d", calls.Load(), len(api.authorizations()), requests)
			}
		})
	}
}

func TestGKELookupDeadlineCancelsTheRequest(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	api := &containerAPI{
		entered:   make(chan struct{}),
		cancelled: make(chan struct{}),
		block:     true,
	}
	api.Server = httptest.NewServer(api)
	t.Cleanup(api.Close)

	for _, on := range []bool{false, true} {
		look := notOnPath
		if on {
			look = onPath
		}
		clock := newFakeClock()
		auth := NewAuthenticator(
			WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
			withLookPath(look),
			withContainerAPIBase(api.URL),
			withLogger(discardLogger(t)),
		)
		auth.clock = clock
		config := gkeIPConfig("https://34.118.10.50", testCertPEM(t), gkeExecConfig())
		result := make(chan error, 1)
		go func() { result <- auth.Apply(config) }()
		select {
		case <-api.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("the lookup request did not start")
		}
		clock.Advance(defaultSetupTimeout)
		select {
		case <-api.cancelled:
		case <-time.After(10 * time.Second):
			t.Fatal("the lookup request context was not cancelled")
		}
		err := <-result
		if on {
			if err != nil {
				t.Fatalf("Apply() with the plugin on PATH error = %v", err)
			}
			if config.ExecProvider == nil || config.WrapTransport != nil {
				t.Fatal("timed-out lookup installed a token")
			}
			continue
		}
		if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("Apply() error = %v, want %s and timed out", err, ErrUntrustedClusterEndpointCode)
		}
	}
}

func TestGKENamedEndpointsDoNotCallTheContainerAPI(t *testing.T) {
	api := startContainerAPI(t, nil)
	var calls atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", &calls)),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
	)
	dns := &rest.Config{Host: testGKEDNSHost, ExecProvider: gkeExecConfig()}
	if err := auth.Apply(dns); err != nil || dns.ExecProvider != nil {
		t.Fatalf("Apply(DNS) = %v, exec %v", err, dns.ExecProvider)
	}
	gateway := &rest.Config{
		Host:         "https://us-central1-connectgateway.googleapis.com/v1/projects/1/locations/us-central1/gkeMemberships/demo",
		ExecProvider: gkeExecConfig(),
	}
	if err := auth.Apply(gateway); err != nil || gateway.ExecProvider != nil {
		t.Fatalf("Apply(Connect Gateway) = %v, exec %v", err, gateway.ExecProvider)
	}
	if len(api.authorizations()) != 0 {
		t.Fatalf("container API calls = %d, want 0", len(api.authorizations()))
	}
	if calls.Load() != 1 {
		t.Fatalf("factory calls = %d, want 1 shared source", calls.Load())
	}
}

func TestGKEAPIRedirectIsNotFollowed(t *testing.T) {
	attacker := newAuthorizationRecorder(t, "adc-token")
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL, http.StatusFound)
	}))
	t.Cleanup(api.Close)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig("https://34.118.10.60", testCertPEM(t), gkeExecConfig())
	err := auth.Apply(config)
	if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("Apply() error = %v", err)
	}
	if attacker.hits.Load() != 0 {
		t.Fatalf("redirect target received %d requests", attacker.hits.Load())
	}
	if config.WrapTransport != nil {
		t.Fatal("token installed after a redirect")
	}
}

func TestGKECAFileIsReadAtVerification(t *testing.T) {
	ca := testCertPEM(t)
	path := t.TempDir() + "/ca.crt"
	if err := os.WriteFile(path, ca, 0o600); err != nil {
		t.Fatal(err)
	}
	const ip = "34.118.10.70"
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(ip, ca),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	config := &rest.Config{
		Host:            "https://" + ip,
		ExecProvider:    gkeExecConfig(),
		TLSClientConfig: rest.TLSClientConfig{CAFile: path},
	}
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("CA file match did not install the token source")
	}
}

func TestGKEUnparseableCADoesNotCallTheAPI(t *testing.T) {
	api := startContainerAPI(t, nil)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig("https://34.118.10.80", []byte("not a certificate"), gkeExecConfig())
	err := auth.Apply(config)
	if errors.GetCode(err) != ErrUntrustedClusterEndpointCode || !strings.Contains(err.Error(), "cannot be parsed") {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(api.authorizations()) != 0 {
		t.Fatal("the Container API was called with an unparseable CA")
	}
}

func TestGKEIPEndpointsAreVerifiedSeparately(t *testing.T) {
	ca := testCertPEM(t)
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor("34.118.10.90", ca),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)
	first := gkeIPConfig("https://34.118.10.90", ca, gkeExecConfig())
	if err := auth.Apply(first); err != nil || first.ExecProvider != nil {
		t.Fatalf("first Apply() = %v, exec %v", err, first.ExecProvider)
	}
	lists := len(api.listProjects())
	second := gkeIPConfig("https://203.0.113.50", ca, gkeExecConfig())
	err := auth.Apply(second)
	if errors.GetCode(err) != ErrUntrustedClusterEndpointCode {
		t.Fatalf("second Apply() error = %v, want a refusal", err)
	}
	if second.WrapTransport != nil {
		t.Fatal("second endpoint reused the first endpoint's verification")
	}
	if len(api.listProjects()) <= lists {
		t.Fatal("second endpoint did not call the Container API")
	}
}

func TestGKE401ResetsAndReverifies(t *testing.T) {
	server := newAuthorizationRecorder(t, "fresh")
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	ip := urlHostIP(t, server.URL)
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(ip, ca),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	log, logs := bufferLogger(t)
	var calls atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(sequenceFactory(&calls, "stale", "fresh")),
		withLookPath(notOnPath),
		withLogger(log),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig(server.URL, ca, gkeExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := getStatus(t, config); got != http.StatusUnauthorized {
		t.Fatalf("first GET status = %d, want 401", got)
	}
	if len(api.listProjects()) != 1 {
		t.Fatalf("list calls after the rejected token = %d, want 1", len(api.listProjects()))
	}
	if got := getStatus(t, config); got != http.StatusOK {
		t.Fatalf("GET after 401 status = %d, want 200", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("factory calls = %d, want 2", calls.Load())
	}
	if len(api.listProjects()) != 2 {
		t.Fatalf("list calls = %d, want 2 (setup after 401 re-verifies)", len(api.listProjects()))
	}
	if lines := strings.Split(strings.TrimSpace(logs.String()), "\n"); len(lines) != 2 {
		t.Fatalf("logged %d lines, want one warning per setup: %s", len(lines), logs.String())
	}
}

func TestInProcessGKEIPWarningIsWrittenToStderr(t *testing.T) {
	ca := testCertPEM(t)
	const ip = "34.118.10.99"
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(ip, ca),
	}})
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = write
	t.Cleanup(func() { os.Stderr = orig })

	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", new(atomic.Int32))),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
	)
	config := gkeIPConfig("https://"+ip, ca, gkeExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "In-process GKE token minting is active for IP endpoint "+ip) {
		t.Fatalf("stderr = %q, want the GKE IP warning", body)
	}
}

func TestPerContextExecOptOut(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("minted", &calls)), withLookPath(notOnPath))

	disabled := gkeExecConfig()
	disabled.Env = []clientcmdapi.ExecEnvVar{{Name: DisableEnvVar, Value: "true"}}
	disabledConfig := &rest.Config{Host: testGKEDNSHost, ExecProvider: disabled}
	if err := auth.Apply(disabledConfig); err != nil {
		t.Fatalf("Apply(disabled) error = %v", err)
	}
	if disabledConfig.ExecProvider != disabled || disabledConfig.WrapTransport != nil || calls.Load() != 0 {
		t.Fatal("a true opt-out was intercepted")
	}

	enabled := gkeExecConfig()
	enabledConfig := &rest.Config{Host: testGKEDNSHost, ExecProvider: enabled}
	if err := auth.Apply(enabledConfig); err != nil {
		t.Fatalf("Apply(enabled) error = %v", err)
	}
	if enabledConfig.ExecProvider != nil || calls.Load() != 1 {
		t.Fatal("another context in the same process was not intercepted")
	}

	falsy := gkeExecConfig()
	falsy.Env = []clientcmdapi.ExecEnvVar{{Name: DisableEnvVar, Value: "false"}}
	falsyConfig := &rest.Config{Host: testGKEDNSHost, ExecProvider: falsy}
	if err := auth.Apply(falsyConfig); err != nil {
		t.Fatalf("Apply(false) error = %v", err)
	}
	if falsyConfig.ExecProvider != nil {
		t.Fatal("a false opt-out blocked interception")
	}

	numeric := gkeExecConfig()
	numeric.Env = []clientcmdapi.ExecEnvVar{{Name: DisableEnvVar, Value: "1"}}
	numericConfig := &rest.Config{Host: testGKEDNSHost, ExecProvider: numeric}
	if err := auth.Apply(numericConfig); err != nil {
		t.Fatalf("Apply(1) error = %v", err)
	}
	if numericConfig.ExecProvider != numeric {
		t.Fatal("ParseBool true value \"1\" was intercepted")
	}
}

func discardLogger(t *testing.T) logger.Handler {
	t.Helper()
	log, _ := bufferLogger(t)
	return log
}
