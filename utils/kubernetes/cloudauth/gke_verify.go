package cloudauth

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// defaultGKEContainerAPI is the GKE Container API origin. Tests replace
	// Authenticator.containerAPIBase with a local server.
	defaultGKEContainerAPI = "https://container.googleapis.com"
	// gkeListPageCap bounds container.clusters.list pagination so a repeated
	// page token cannot loop for the rest of the setup deadline.
	gkeListPageCap = 20
)

// gkeNoMatchCause is the cause used when the API answered but no cluster had
// both the kubeconfig IP and the kubeconfig CA.
const gkeNoMatchCause = "no cluster matched the kubeconfig IP and CA certificate"

// gkeProjectIDPattern matches a Google Cloud project id: it starts with a
// letter, then lowercase letters, digits and hyphens, and is 6 to 30
// characters. The gke_ context name uses this form.
var gkeProjectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

// gkeProjectNumberPattern matches a numeric project id, which the Container
// API accepts in list URLs and which Application Default Credentials may
// report. The gke_ name uses the project id, not the number.
var gkeProjectNumberPattern = regexp.MustCompile(`^[0-9]{6,20}$`)

// gkeLocationPattern matches a GKE region (us-central1, europe-west10,
// northamerica-northeast1) or zone (us-central1-a, europe-west12-b).
var gkeLocationPattern = regexp.MustCompile(`^[a-z]+(?:-[a-z]+)*[0-9]+(?:-[a-z])?$`)

// gkeClusterNamePattern matches a GKE cluster name. It may contain hyphens.
// Underscores are not part of a cluster name; they separate the gke_ hint.
var gkeClusterNamePattern = regexp.MustCompile(`^[a-z](?:[a-z0-9-]{0,38}[a-z0-9])?$`)

// gkeNamedCluster is a project, location and cluster parsed from a
// kubeconfig context or cluster name. It is only a hint.
type gkeNamedCluster struct {
	Project  string
	Location string
	Cluster  string
}

// gkeClusterIdentity is the cluster the Container API confirmed.
type gkeClusterIdentity struct {
	Project  string
	Location string
	Name     string
}

// gkeCluster is the subset of a Container API Cluster used to compare an
// endpoint and a CA certificate.
type gkeCluster struct {
	Name       string `json:"name"`
	Location   string `json:"location"`
	Endpoint   string `json:"endpoint"`
	MasterAuth struct {
		ClusterCaCertificate string `json:"clusterCaCertificate"`
	} `json:"masterAuth"`
	PrivateClusterConfig struct {
		PublicEndpoint  string `json:"publicEndpoint"`
		PrivateEndpoint string `json:"privateEndpoint"`
	} `json:"privateClusterConfig"`
	ControlPlaneEndpointsConfig struct {
		IPEndpointsConfig struct {
			PublicEndpoint  string `json:"publicEndpoint"`
			PrivateEndpoint string `json:"privateEndpoint"`
		} `json:"ipEndpointsConfig"`
	} `json:"controlPlaneEndpointsConfig"`
}

type gkeListResponse struct {
	Clusters      []gkeCluster `json:"clusters"`
	NextPageToken string       `json:"nextPageToken"`
}

// adcProjectID returns the project id from Application Default Credentials.
// Tests replace it so lookup does not probe the metadata server.
var adcProjectID = defaultADCProjectID

func defaultADCProjectID(ctx context.Context) string {
	creds, err := google.FindDefaultCredentials(ctx, gkeScopes...)
	if err != nil || creds == nil {
		return ""
	}
	return creds.ProjectID
}

// parseGKEClusterName parses a kubeconfig context or cluster name of the
// form gke_<project>_<location>_<cluster>. The cluster part is everything
// after the third underscore, so a hyphenated cluster name stays intact.
// The name is a hint for container.clusters.get; a parse failure only means
// the lookup starts at container.clusters.list.
func parseGKEClusterName(name string) (gkeNamedCluster, bool) {
	rest, ok := strings.CutPrefix(name, "gke_")
	if !ok || rest == "" {
		return gkeNamedCluster{}, false
	}
	parts := strings.SplitN(rest, "_", 3)
	if len(parts) != 3 {
		return gkeNamedCluster{}, false
	}
	named := gkeNamedCluster{Project: parts[0], Location: parts[1], Cluster: parts[2]}
	if !gkeProjectIDPattern.MatchString(named.Project) ||
		!gkeLocationPattern.MatchString(named.Location) ||
		!gkeClusterNamePattern.MatchString(named.Cluster) {
		return gkeNamedCluster{}, false
	}
	return named, true
}

// gkeNamesFromHint returns the name to pass to container.clusters.get, if
// either hint parses, and every distinct parsed name. The context name wins
// over the cluster name when both parse.
func gkeNamesFromHint(contextName, clusterName string) (get *gkeNamedCluster, parsed []gkeNamedCluster) {
	var first *gkeNamedCluster
	add := func(name string) {
		named, ok := parseGKEClusterName(name)
		if !ok {
			return
		}
		for _, existing := range parsed {
			if existing == named {
				if first == nil {
					copyNamed := named
					first = &copyNamed
				}
				return
			}
		}
		parsed = append(parsed, named)
		if first == nil {
			copyNamed := named
			first = &copyNamed
		}
	}
	add(contextName)
	add(clusterName)
	return first, parsed
}

func validGKEProject(project string) bool {
	return gkeProjectIDPattern.MatchString(project) || gkeProjectNumberPattern.MatchString(project)
}

// gkeProjectIDs returns the projects to pass to container.clusters.list, in
// order, without duplicates: projects parsed from the name hint, the ADC
// project, then GOOGLE_CLOUD_PROJECT and GOOGLE_PROJECT.
func (a *Authenticator) gkeProjectIDs(ctx context.Context, parsed []gkeNamedCluster) []string {
	var projects []string
	add := func(project string) {
		project = strings.TrimSpace(project)
		if !validGKEProject(project) {
			return
		}
		for _, existing := range projects {
			if existing == project {
				return
			}
		}
		projects = append(projects, project)
	}
	for _, named := range parsed {
		add(named.Project)
	}
	add(adcProjectID(ctx))
	add(a.getenv("GOOGLE_CLOUD_PROJECT"))
	add(a.getenv("GOOGLE_PROJECT"))
	return projects
}

func (g *gkeIPEndpoint) caBytes() ([]byte, error) {
	if len(g.CA) > 0 {
		return g.CA, nil
	}
	if g.CAFile == "" {
		return nil, fmt.Errorf("GKE API lookup failed: the kubeconfig has no certificate-authority")
	}
	data, err := os.ReadFile(g.CAFile)
	if err != nil {
		return nil, fmt.Errorf("GKE API lookup failed: cannot read the certificate-authority file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("GKE API lookup failed: the certificate-authority file is empty")
	}
	return data, nil
}

// verifyGKEIPEndpoint reports the cluster whose control plane IP and CA
// certificate match check. It sends the token from source to the Container
// API only. It does not contact check.Host.
func verifyGKEIPEndpoint(ctx context.Context, a *Authenticator, source oauth2.TokenSource, check *gkeIPEndpoint) (gkeClusterIdentity, error) {
	caBytes, err := check.caBytes()
	if err != nil {
		return gkeClusterIdentity{}, err
	}
	wantCA, err := certificateDERSet(caBytes)
	if err != nil {
		return gkeClusterIdentity{}, fmt.Errorf("GKE API lookup failed: the kubeconfig CA certificate cannot be parsed")
	}
	wantIP := net.ParseIP(check.Host)
	if wantIP == nil {
		return gkeClusterIdentity{}, fmt.Errorf("GKE API lookup failed: %q is not an IP address", check.Host)
	}

	client := gkeAPIClient(source)
	get, parsed := gkeNamesFromHint(check.ContextName, check.ClusterName)
	var outcome gkeLookup

	if get != nil {
		var cluster gkeCluster
		status, err := a.gkeGET(ctx, client, clusterGetPath(*get), &cluster)
		if err != nil {
			return gkeClusterIdentity{}, err
		}
		if status == http.StatusOK && clusterMatches(cluster, wantIP, wantCA) {
			return identityFrom(get.Project, get.Location, get.Cluster, cluster), nil
		}
		outcome.note(get.Project, status)
	}

	projects := a.gkeProjectIDs(ctx, parsed)
	if get == nil && len(projects) == 0 {
		return gkeClusterIdentity{}, fmt.Errorf("GKE API lookup failed: no project to search; name the cluster gke_<project>_<location>_<cluster> or set GOOGLE_CLOUD_PROJECT")
	}
	for _, project := range projects {
		clusters, err := a.listGKEClusters(ctx, client, project, &outcome)
		if err != nil {
			return gkeClusterIdentity{}, err
		}
		for _, cluster := range clusters {
			if clusterMatches(cluster, wantIP, wantCA) {
				return identityFrom(project, cluster.Location, shortClusterName(cluster.Name), cluster), nil
			}
		}
	}
	return gkeClusterIdentity{}, outcome.err()
}

func identityFrom(project, location, name string, cluster gkeCluster) gkeClusterIdentity {
	if cluster.Location != "" {
		location = cluster.Location
	}
	if short := shortClusterName(cluster.Name); short != "" {
		name = short
	}
	return gkeClusterIdentity{Project: project, Location: location, Name: name}
}

func shortClusterName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// gkeLookup records the Container API answers a lookup saw, per project. A
// 200 answers only for its own project: it hides an earlier 403 from the
// same project, but not a denial or error from another project.
type gkeLookup struct {
	answered map[string]bool
	refused  []gkeRefusal
}

type gkeRefusal struct {
	project string
	status  int
}

func (l *gkeLookup) note(project string, status int) {
	switch status {
	case http.StatusOK:
		if l.answered == nil {
			l.answered = map[string]bool{}
		}
		l.answered[project] = true
	case http.StatusNotFound:
	default:
		l.refused = append(l.refused, gkeRefusal{project: project, status: status})
	}
}

func (l *gkeLookup) err() error {
	var denied, failed []string
	for _, refusal := range l.refused {
		if l.answered[refusal.project] {
			continue
		}
		if refusal.status == http.StatusUnauthorized || refusal.status == http.StatusForbidden {
			if !slices.Contains(denied, refusal.project) {
				denied = append(denied, refusal.project)
			}
			continue
		}
		cause := fmt.Sprintf("the GKE API returned HTTP %d for project %s", refusal.status, refusal.project)
		if !slices.Contains(failed, cause) {
			failed = append(failed, cause)
		}
	}
	var causes []string
	if len(l.answered) > 0 || (len(denied) == 0 && len(failed) == 0) {
		causes = append(causes, gkeNoMatchCause)
	}
	if len(denied) > 0 {
		causes = append(causes, "permission denied for project "+strings.Join(denied, ", ")+
			" (requires container.clusters.get or container.clusters.list)")
	}
	causes = append(causes, failed...)
	return errors.New("GKE API lookup failed: " + strings.Join(causes, "; "))
}

func clusterGetPath(named gkeNamedCluster) string {
	return "/v1/projects/" + url.PathEscape(named.Project) +
		"/locations/" + url.PathEscape(named.Location) +
		"/clusters/" + url.PathEscape(named.Cluster)
}

func clusterListPath(project, pageToken string) string {
	path := "/v1/projects/" + url.PathEscape(project) + "/locations/-/clusters"
	if pageToken == "" {
		return path
	}
	return path + "?pageToken=" + url.QueryEscape(pageToken)
}

func (a *Authenticator) containerAPIURL(pathAndQuery string) string {
	base := strings.TrimRight(a.containerAPIBase, "/")
	if base == "" {
		base = defaultGKEContainerAPI
	}
	return base + pathAndQuery
}

// gkeAPIClient calls the Container API with source's token and refuses
// redirects, so a 3xx cannot carry that token to the kubeconfig host.
func gkeAPIClient(source oauth2.TokenSource) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{Source: source},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// gkeGET performs one Container API GET. A non-2xx status is returned with
// a nil error so the caller can fall through to list; transport failures
// and deadlines are errors and stop the lookup.
func (a *Authenticator) gkeGET(ctx context.Context, client *http.Client, path string, dest any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.containerAPIURL(path), nil)
	if err != nil {
		return 0, fmt.Errorf("GKE API lookup failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, fmt.Errorf("GKE API lookup failed: timed out")
		}
		return 0, fmt.Errorf("GKE API lookup failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		if ctx.Err() != nil {
			return 0, fmt.Errorf("GKE API lookup failed: timed out")
		}
		return resp.StatusCode, fmt.Errorf("GKE API lookup failed: %w", err)
	}
	if resp.StatusCode == http.StatusOK && dest != nil && len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, dest); err != nil {
			return resp.StatusCode, fmt.Errorf("GKE API lookup failed: the GKE API response could not be read: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (a *Authenticator) listGKEClusters(ctx context.Context, client *http.Client, project string, outcome *gkeLookup) ([]gkeCluster, error) {
	var all []gkeCluster
	page := ""
	seen := map[string]struct{}{}
	for pages := 0; pages < gkeListPageCap; pages++ {
		if _, dup := seen[page]; dup && pages > 0 {
			break
		}
		seen[page] = struct{}{}
		var body gkeListResponse
		status, err := a.gkeGET(ctx, client, clusterListPath(project, page), &body)
		if err != nil {
			return nil, err
		}
		outcome.note(project, status)
		if status != http.StatusOK {
			return all, nil
		}
		all = append(all, body.Clusters...)
		if body.NextPageToken == "" {
			return all, nil
		}
		page = body.NextPageToken
	}
	return all, nil
}

func clusterMatches(cluster gkeCluster, wantIP net.IP, wantCA map[string]struct{}) bool {
	if !clusterHasIP(cluster, wantIP) {
		return false
	}
	gotCA, err := certificateDERSet([]byte(cluster.MasterAuth.ClusterCaCertificate))
	if err != nil {
		return false
	}
	return derSetsEqual(wantCA, gotCA)
}

func clusterHasIP(cluster gkeCluster, want net.IP) bool {
	for _, raw := range []string{
		cluster.Endpoint,
		cluster.PrivateClusterConfig.PublicEndpoint,
		cluster.PrivateClusterConfig.PrivateEndpoint,
		cluster.ControlPlaneEndpointsConfig.IPEndpointsConfig.PublicEndpoint,
		cluster.ControlPlaneEndpointsConfig.IPEndpointsConfig.PrivateEndpoint,
	} {
		if ip := parseEndpointIP(raw); ip != nil && ip.Equal(want) {
			return true
		}
	}
	return false
}

// parseEndpointIP returns the IP in raw, ignoring a port and brackets.
// Host names and empty values return nil.
func parseEndpointIP(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	return net.ParseIP(raw)
}

// certificateDERSet parses data as PEM, as base64 of PEM or DER, or as a
// single DER certificate, and returns the set of certificate DER encodings.
// PEM and base64 forms of the same certificates compare equal.
func certificateDERSet(data []byte) (map[string]struct{}, error) {
	ders, err := certificateDERs(data)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(ders))
	for _, der := range ders {
		set[string(der)] = struct{}{}
	}
	return set, nil
}

func derSetsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for der := range a {
		if _, ok := b[der]; !ok {
			return false
		}
	}
	return true
}

func certificateDERs(data []byte) ([][]byte, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty certificate")
	}
	if bytes.Contains(data, []byte("-----BEGIN")) {
		return pemCertificateDERs(data)
	}
	compact := stripCertSpace(data)
	decoded, err := base64.StdEncoding.DecodeString(string(compact))
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(string(compact))
	}
	if err == nil && len(decoded) > 0 {
		if bytes.Contains(decoded, []byte("-----BEGIN")) {
			return pemCertificateDERs(decoded)
		}
		if ders, derErr := derCertificates(decoded); derErr == nil {
			return ders, nil
		}
	}
	return derCertificates(data)
}

func pemCertificateDERs(data []byte) ([][]byte, error) {
	var ders [][]byte
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		ders = append(ders, cert.Raw)
	}
	if len(ders) == 0 {
		return nil, fmt.Errorf("no certificates")
	}
	return ders, nil
}

func derCertificates(data []byte) ([][]byte, error) {
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, err
	}
	return [][]byte{cert.Raw}, nil
}

func stripCertSpace(data []byte) []byte {
	return bytes.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, data)
}
