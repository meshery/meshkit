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

// defaultGKEContainerAPI is the GKE Container API origin. Tests replace
// Authenticator.containerAPIBase with a local server.
const defaultGKEContainerAPI = "https://container.googleapis.com"

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
// kubeconfig context or cluster name. It is only a hint, and it is used only
// when its project is trusted.
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

// gkeListResponse is a container.clusters.list response. The method is not
// paginated. MissingZones names zones the API could not reach, so a list
// that has them is partial.
type gkeListResponse struct {
	Clusters     []gkeCluster `json:"clusters"`
	MissingZones []string     `json:"missingZones"`
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
// The name is a hint for container.clusters.get in a trusted project; a
// parse failure only means the lookup starts at container.clusters.list.
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

// gkeMaxHints bounds the gke_ names kept for one GKE IP endpoint, and so the
// container.clusters.get calls in one setup.
const gkeMaxHints = 4

// gkeEndpointHints appends the distinct gke_<project>_<location>_<cluster>
// names among check's context and cluster names to hints, oldest first, and
// keeps only the gkeMaxHints most recent.
func gkeEndpointHints(hints []gkeNamedCluster, check *gkeIPEndpoint) []gkeNamedCluster {
	for _, name := range []string{check.ContextName, check.ClusterName} {
		named, ok := parseGKEClusterName(name)
		if ok && !slices.Contains(hints, named) {
			hints = append(hints, named)
		}
	}
	if len(hints) > gkeMaxHints {
		hints = slices.Delete(hints, 0, len(hints)-gkeMaxHints)
	}
	return hints
}

// gkeHintedClusters returns the hints with a trusted project, most recent
// first, for container.clusters.get. When no hint has one, untrusted is the
// project of the most recent hint; that project is never queried.
func gkeHintedClusters(hints []gkeNamedCluster, trusted []string) (gets []gkeNamedCluster, untrusted string) {
	for i := len(hints) - 1; i >= 0; i-- {
		named := hints[i]
		if slices.Contains(trusted, named.Project) {
			gets = append(gets, named)
		} else if untrusted == "" {
			untrusted = named.Project
		}
	}
	if len(gets) > 0 {
		untrusted = ""
	}
	return gets, untrusted
}

func validGKEProject(project string) bool {
	return gkeProjectIDPattern.MatchString(project) || gkeProjectNumberPattern.MatchString(project)
}

// gkeTrustedProjects returns the projects the lookup may search, in order,
// without duplicates: the ADC project, then GOOGLE_CLOUD_PROJECT and
// GOOGLE_PROJECT. The operator of the Meshery process chooses them; the
// kubeconfig cannot add one.
func (a *Authenticator) gkeTrustedProjects(ctx context.Context) []string {
	var projects []string
	for _, project := range []string{adcProjectID(ctx), a.getenv("GOOGLE_CLOUD_PROJECT"), a.getenv("GOOGLE_PROJECT")} {
		project = strings.TrimSpace(project)
		if validGKEProject(project) && !slices.Contains(projects, project) {
			projects = append(projects, project)
		}
	}
	return projects
}

// gkeUntrustedHintCause explains that the kubeconfig named a project outside
// the trusted set, so a refusal does not look like a missing cluster.
func gkeUntrustedHintCause(project string) string {
	return "the kubeconfig names project " + project + ", which is not the ADC project, GOOGLE_CLOUD_PROJECT or GOOGLE_PROJECT, so it was not searched"
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
// certificate match check. hints are the gke_ names seen for check's IP and
// CA; each one in a trusted project gets a container.clusters.get call, most
// recent first, before the trusted projects are listed. It sends the token
// from source to the Container API only. It does not contact check.Host.
func verifyGKEIPEndpoint(ctx context.Context, a *Authenticator, source oauth2.TokenSource, check *gkeIPEndpoint, hints []gkeNamedCluster) (gkeClusterIdentity, error) {
	caBytes, err := check.caBytes()
	if err != nil {
		return gkeClusterIdentity{}, err
	}
	wantCA, err := pemCertificateSet(caBytes)
	if err != nil {
		return gkeClusterIdentity{}, fmt.Errorf("GKE API lookup failed: the kubeconfig CA certificate cannot be parsed as PEM")
	}
	wantIP := net.ParseIP(check.Host)
	if wantIP == nil {
		return gkeClusterIdentity{}, fmt.Errorf("GKE API lookup failed: %q is not an IP address", check.Host)
	}

	trusted := a.gkeTrustedProjects(ctx)
	gets, untrusted := gkeHintedClusters(hints, trusted)
	if len(trusted) == 0 {
		cause := "GKE API lookup failed: no trusted project to search; set GOOGLE_CLOUD_PROJECT or use Application Default Credentials that name a project"
		if untrusted != "" {
			cause += "; " + gkeUntrustedHintCause(untrusted)
		}
		return gkeClusterIdentity{}, errors.New(cause)
	}

	client := gkeAPIClient(source)
	outcome := gkeLookup{untrusted: untrusted}
	for _, get := range gets {
		var cluster gkeCluster
		status, err := a.gkeGET(ctx, client, clusterGetPath(get), &cluster)
		if err != nil {
			return gkeClusterIdentity{}, err
		}
		if status == http.StatusOK && clusterMatches(cluster, wantIP, wantCA) {
			return identityFrom(get.Project, get.Location, get.Cluster, cluster), nil
		}
		outcome.note(get.Project, status)
	}

	for _, project := range trusted {
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
// same project, but not a denial or error from another project. A partial
// list (missingZones) is never a complete answer.
type gkeLookup struct {
	answered  map[string]bool
	refused   []gkeRefusal
	partial   []string
	untrusted string
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

func (l *gkeLookup) notePartial(project string, missingZones []string) {
	l.partial = append(l.partial, "the GKE API could not list zones "+strings.Join(missingZones, ", ")+" for project "+project)
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
	if len(l.answered) > 0 {
		causes = append(causes, gkeNoMatchCause)
	}
	if len(denied) > 0 {
		causes = append(causes, "permission denied for project "+strings.Join(denied, ", ")+
			" (requires container.clusters.get or container.clusters.list)")
	}
	causes = append(causes, failed...)
	causes = append(causes, l.partial...)
	if len(causes) == 0 {
		causes = append(causes, gkeNoMatchCause)
	}
	if l.untrusted != "" {
		causes = append(causes, gkeUntrustedHintCause(l.untrusted))
	}
	return errors.New("GKE API lookup failed: " + strings.Join(causes, "; "))
}

func clusterGetPath(named gkeNamedCluster) string {
	return "/v1/projects/" + url.PathEscape(named.Project) +
		"/locations/" + url.PathEscape(named.Location) +
		"/clusters/" + url.PathEscape(named.Cluster)
}

func clusterListPath(project string) string {
	return "/v1/projects/" + url.PathEscape(project) + "/locations/-/clusters"
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

// listGKEClusters returns the clusters container.clusters.list reports for
// project in every location. A list with missingZones is recorded as
// partial, so a lookup that finds no match in it is not reported as a
// definitive no-match.
func (a *Authenticator) listGKEClusters(ctx context.Context, client *http.Client, project string, outcome *gkeLookup) ([]gkeCluster, error) {
	var body gkeListResponse
	status, err := a.gkeGET(ctx, client, clusterListPath(project), &body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		outcome.note(project, status)
		return nil, nil
	}
	if len(body.MissingZones) > 0 {
		outcome.notePartial(project, body.MissingZones)
	} else {
		outcome.note(project, status)
	}
	return body.Clusters, nil
}

func clusterMatches(cluster gkeCluster, wantIP net.IP, wantCA map[string]struct{}) bool {
	if !clusterHasIP(cluster, wantIP) {
		return false
	}
	gotCA, err := apiCertificateSet(cluster.MasterAuth.ClusterCaCertificate)
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

// pemCertificateSet parses the kubeconfig CA, which client-go accepts only
// as PEM, and returns the set of its certificate DER encodings.
func pemCertificateSet(data []byte) (map[string]struct{}, error) {
	set := map[string]struct{}{}
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
		set[string(cert.Raw)] = struct{}{}
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("no PEM certificates")
	}
	return set, nil
}

// apiCertificateSet parses masterAuth.clusterCaCertificate, which the
// Container API returns as standard base64 of PEM.
func apiCertificateSet(value string) (map[string]struct{}, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return pemCertificateSet(decoded)
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
