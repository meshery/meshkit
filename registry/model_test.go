package registry

import (
	"strings"
	"testing"
)

func TestSanitizeDocsURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "already a slug is left alone",
			raw:  "https://docs.meshery.io/extensibility/integrations/aad-pod-identity",
			want: "https://docs.meshery.io/extensibility/integrations/aad-pod-identity",
		},
		{
			name: "display name becomes the published slug",
			raw:  "https://docs.meshery.io/extensibility/integrations/Azure Kubernetes Service",
			want: "https://docs.meshery.io/extensibility/integrations/azure-kubernetes-service",
		},
		{
			name: "lowercase display name becomes the published slug",
			raw:  "https://docs.meshery.io/extensibility/integrations/confidential containers",
			want: "https://docs.meshery.io/extensibility/integrations/confidential-containers",
		},
		{
			name: "parentheses in a slug survive",
			raw:  "https://docs.meshery.io/extensibility/integrations/open policy agent (opa)",
			want: "https://docs.meshery.io/extensibility/integrations/open-policy-agent-(opa)",
		},
		{
			name: "trailing whitespace is dropped",
			raw:  "https://docs.meshery.io/installation/docker ",
			want: "https://docs.meshery.io/installation/docker",
		},
		{
			name: "repeated separators collapse",
			raw:  "https://docs.meshery.io/installation//windows",
			want: "https://docs.meshery.io/installation/windows",
		},
		{
			name: "nested installation paths are untouched",
			raw:  "https://docs.meshery.io/installation/kubernetes/eks",
			want: "https://docs.meshery.io/installation/kubernetes/eks",
		},
		{
			name: "trailing slash is kept",
			raw:  "https://docs.meshery.io/extensibility/integrations/istio/",
			want: "https://docs.meshery.io/extensibility/integrations/istio/",
		},
		{
			name: "fragment keeps its case",
			raw:  "https://docs.meshery.io/installation/kubernetes#Helm",
			want: "https://docs.meshery.io/installation/kubernetes#Helm",
		},
		{
			name: "query string keeps its case",
			raw:  "https://docs.meshery.io/extensibility/integrations?filter=Istio",
			want: "https://docs.meshery.io/extensibility/integrations?filter=Istio",
		},
		{
			name: "another host keeps its path as authored",
			raw:  " https://github.com/meshery/Meshery/tree/master/Docs ",
			want: "https://github.com/meshery/Meshery/tree/master/Docs",
		},
		{
			name: "host-only URL is left alone",
			raw:  "https://docs.meshery.io",
			want: "https://docs.meshery.io",
		},
		{
			name: "value that is not a URL is only trimmed",
			raw:  "  see the Istio adapter docs  ",
			want: "see the Istio adapter docs",
		},
		{
			name: "empty value stays empty",
			raw:  "   ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeDocsURL(tt.raw); got != tt.want {
				t.Errorf("sanitizeDocsURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSanitizeDocsURLIsIdempotent(t *testing.T) {
	raw := "https://docs.meshery.io/extensibility/integrations/Open Cluster Management"

	once := sanitizeDocsURL(raw)
	if twice := sanitizeDocsURL(once); twice != once {
		t.Errorf("sanitizeDocsURL is not idempotent: %q then %q", once, twice)
	}
}

func TestCreateMarkDownForMDStyleSanitizesDocsURL(t *testing.T) {
	model := ModelCSV{
		Model:            "azure kubernetes service",
		ModelDisplayName: "Azure Kubernetes Service",
		DocsURL:          "https://docs.meshery.io/extensibility/integrations/Azure Kubernetes Service",
	}

	for _, outputFor := range []string{"mesherydocs", "mesheryio"} {
		t.Run(outputFor, func(t *testing.T) {
			markdown := model.CreateMarkDownForMDStyle("", "", 0, 0, outputFor)
			want := "docURL: https://docs.meshery.io/extensibility/integrations/azure-kubernetes-service\n"

			if !strings.Contains(markdown, want) {
				t.Errorf("generated %s page does not carry %q:\n%s", outputFor, want, markdown)
			}
		})
	}
}

func TestCreateMarkDownForMDXStyleSanitizesDocsURL(t *testing.T) {
	model := ModelCSV{
		Model:            "open cluster management",
		ModelDisplayName: "Open Cluster Management",
		DocsURL:          "https://docs.meshery.io/extensibility/integrations/open cluster management",
	}

	mdx := model.CreateMarkDownForMDXStyle("")
	want := "docURL: https://docs.meshery.io/extensibility/integrations/open-cluster-management\n"

	if !strings.Contains(mdx, want) {
		t.Errorf("generated MDX page does not carry %q:\n%s", want, mdx)
	}
}

func TestCreateJSONItemSanitizesPermalink(t *testing.T) {
	model := ModelCSV{
		Model:   "piraeus datastore",
		DocsURL: "https://docs.meshery.io/extensibility/integrations/piraeus datastore",
	}

	item := model.CreateJSONItem("assets/images/integration")
	want := `"permalink":"https://docs.meshery.io/extensibility/integrations/piraeus-datastore"`

	if !strings.Contains(item, want) {
		t.Errorf("generated catalog item does not carry %s:\n%s", want, item)
	}
}
