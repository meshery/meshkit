package cloudauth

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

var testCAData = []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")

func TestBindEndpointEKS(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		region     string
		insecure   bool
		proxy      bool
		wantRegion string
		wantErr    string
	}{
		{name: "regional endpoint", host: testEKSHost, region: "us-west-2", wantRegion: "us-west-2"},
		{name: "region taken from the endpoint", host: testEKSHost, wantRegion: "us-west-2"},
		{name: "older yl4 cell", host: "https://ABCDEF0123456789.yl4.eu-central-1.eks.amazonaws.com", wantRegion: "eu-central-1"},
		{name: "endpoint without a cell label", host: "https://ABCDEF0123456789.sa-east-1.eks.amazonaws.com", wantRegion: "sa-east-1"},
		{name: "GovCloud", host: "https://ABCDEF0123456789.gr7.us-gov-west-1.eks.amazonaws.com", wantRegion: "us-gov-west-1"},
		{name: "China partition", host: "https://ABCDEF0123456789.gr7.cn-north-1.eks.amazonaws.com.cn", wantRegion: "cn-north-1"},
		{name: "IPv6 endpoint", host: "https://ABCDEF0123456789.us-west-2.api.aws", wantRegion: "us-west-2"},
		{name: "IPv6 endpoint in China", host: "https://ABCDEF0123456789.cn-north-1.api.amazonwebservices.com.cn", wantRegion: "cn-north-1"},
		{name: "unrelated api.aws host without a region", host: "https://foo.example.api.aws", wantErr: "is not an EKS API endpoint"},
		{name: "explicit port and trailing dot", host: "https://ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com.:443/", wantRegion: "us-east-1"},
		{name: "region mismatch", host: testEKSHost, region: "eu-west-1", wantErr: `mints for region "eu-west-1"`},
		{name: "attacker host", host: "https://attacker.example.com", wantErr: "is not an EKS API endpoint"},
		{name: "attacker host under the EKS name", host: "https://evil.eks.amazonaws.com.attacker.example", wantErr: "is not an EKS API endpoint"},
		{name: "attacker subdomain of eks.amazonaws.com", host: "https://a.b.c.us-west-2.eks.amazonaws.com", wantErr: "is not an EKS API endpoint"},
		{name: "not a region", host: "https://ABCDEF.gr7.notaregion.eks.amazonaws.com", wantErr: "is not an EKS API endpoint"},
		{name: "IP address", host: "https://203.0.113.10", wantErr: "is not an EKS API endpoint"},
		{name: "http scheme", host: "http://ABCDEF.gr7.us-west-2.eks.amazonaws.com", wantErr: "does not use https"},
		{name: "no scheme", host: "ABCDEF.gr7.us-west-2.eks.amazonaws.com", wantErr: "does not use https"},
		{name: "TLS verification disabled", host: testEKSHost, insecure: true, wantErr: "insecure-skip-tls-verify"},
		{name: "kubeconfig proxy", host: testEKSHost, proxy: true, wantErr: "proxy-url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &rest.Config{Host: tt.host, TLSClientConfig: rest.TLSClientConfig{Insecure: tt.insecure}}
			if tt.proxy {
				config.Proxy = func(*http.Request) (*url.URL, error) { return url.Parse("http://proxy.example.com:3128") }
			}
			spec := Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo", Region: tt.region}}
			got, err := bindEndpoint(config, spec)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("bindEndpoint() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("bindEndpoint() error = %v", err)
			}
			if got.EKS.Region != tt.wantRegion {
				t.Fatalf("bound region = %q, want %q", got.EKS.Region, tt.wantRegion)
			}
		})
	}
}

func TestBindEndpointGKE(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		caData   []byte
		caFile   string
		insecure bool
		wantErr  string
	}{
		{name: "public IP endpoint with CA data", host: "https://34.118.10.20", caData: testCAData},
		{name: "private IP endpoint with CA file", host: "https://10.0.0.2:443", caFile: "/etc/kube/ca.crt"},
		{name: "IPv6 endpoint with CA data", host: "https://[2600:1900:4000::1]", caData: testCAData},
		{name: "DNS-based endpoint", host: testGKEDNSHost},
		{name: "global Connect Gateway", host: "https://connectgateway.googleapis.com/v1/projects/123/locations/global/gkeMemberships/demo"},
		{name: "regional Connect Gateway", host: "https://us-central1-connectgateway.googleapis.com/v1/projects/123/locations/us-central1/gkeMemberships/demo"},
		{name: "IP endpoint without CA", host: "https://34.118.10.20", wantErr: "has no certificate-authority"},
		{name: "attacker host", host: "https://attacker.example.com", caData: testCAData, wantErr: "is not a GKE control plane endpoint"},
		{name: "bare gke.goog", host: "https://gke.goog", wantErr: "is not a GKE control plane endpoint"},
		{name: "gke.goog lookalike", host: "https://evilgke.goog", wantErr: "is not a GKE control plane endpoint"},
		{name: "gke.goog prefix of an attacker domain", host: "https://x.gke.goog.attacker.example", wantErr: "is not a GKE control plane endpoint"},
		{name: "Connect Gateway prefix of an attacker domain", host: "https://connectgateway.googleapis.com.attacker.example", wantErr: "is not a GKE control plane endpoint"},
		{name: "other googleapis host", host: "https://storage.googleapis.com", wantErr: "is not a GKE control plane endpoint"},
		{name: "http scheme", host: "http://34.118.10.20", caData: testCAData, wantErr: "does not use https"},
		{name: "TLS verification disabled", host: "https://34.118.10.20", caData: testCAData, insecure: true, wantErr: "insecure-skip-tls-verify"},
		{name: "TLS verification disabled for a DNS endpoint", host: testGKEDNSHost, insecure: true, wantErr: "insecure-skip-tls-verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &rest.Config{Host: tt.host, TLSClientConfig: rest.TLSClientConfig{
				CAData: tt.caData, CAFile: tt.caFile, Insecure: tt.insecure,
			}}
			_, err := bindEndpoint(config, Spec{Provider: ProviderGKE, Command: "gke-gcloud-auth-plugin"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("bindEndpoint() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("bindEndpoint() error = %v", err)
			}
		})
	}
}
