package component

import (
	"testing"

	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1/component"
)

var istioCrd = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    "helm.sh/resource-policy": keep
  labels:
    app: istio-pilot
    chart: istio
    heritage: Tiller
    release: istio
  name: wasmplugins.extensions.istio.io
spec:
  group: extensions.istio.io
  names:
    categories:
    - istio-io
    - extensions-istio-io
    kind: WasmPlugin
    listKind: WasmPluginList
    plural: wasmplugins
    singular: wasmplugin
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: 'CreationTimestamp is a timestamp representing the server time
        when this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC. Populated by the system. Read-only. Null for
        lists. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata'
      jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1beta1
    schema:
      openAPIV3Schema:
        properties:
          spec:
            description: 'Extend the functionality provided by the Istio proxy through
              WebAssembly filters. See more details at: https://istio.io/docs/reference/config/proxy_extensions/wasm-plugin.html'
            properties:
              imagePullPolicy:
                enum:
                - UNSPECIFIED_POLICY
                - IfNotPresent
                - Always
                type: string
              imagePullSecret:
                description: Credentials to use for OCI image pulling.
                type: string
              phase:
                description: Determines where in the filter chain this WasmPlugin
                  is to be injected.
                enum:
                - UNSPECIFIED_PHASE
                - AUTHN
                - AUTHZ
                - STATS
                type: string
              pluginConfig:
                description: The configuration that will be passed on to the plugin.
                type: object
                x-kubernetes-preserve-unknown-fields: true
              pluginName:
                type: string
              priority:
                description: Determines ordering of WasmPlugins in the same phase.
                nullable: true
                type: integer
              selector:
                properties:
                  matchLabels:
                    additionalProperties:
                      type: string
                    type: object
                type: object
              sha256:
                description: SHA256 checksum that will be used to verify Wasm module
                  or OCI container.
                type: string
              url:
                description: URL of a Wasm module or OCI container.
                type: string
              verificationKey:
                type: string
              vmConfig:
                description: Configuration for a Wasm VM.
                properties:
                  env:
                    description: Specifies environment variables to be injected to
                      this VM.
                    items:
                      properties:
                        name:
                          type: string
                        value:
                          description: Value for the environment variable.
                          type: string
                        valueFrom:
                          enum:
                          - INLINE
                          - HOST
                          type: string
                      type: object
                    type: array
                type: object
            type: object
          status:
            type: object
            x-kubernetes-preserve-unknown-fields: true
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`

func getNewComponent(spec string, name string, version string) component.ComponentDefinition {
	comp := component.ComponentDefinition{}
	comp.Component.Schema = spec
	comp.DisplayName = manifests.FormatToReadableString(name)
	comp.Component.Version = version
	comp.Component.Kind = name
	return comp
}

func TestGenerate(t *testing.T) {
	var tests = []struct {
		crd  string
		want component.ComponentDefinition
	}{
		{istioCrd, getNewComponent("", "WasmPlugin", "extensions.istio.io/v1beta1")},
	}
	for _, tt := range tests {
		t.Run("generateComponent", func(t *testing.T) {
			got, _ := Generate(tt.crd)
			if got.DisplayName != tt.want.DisplayName {
				t.Errorf("got %v, want %v", got.DisplayName, tt.want.DisplayName)
			}
			if !(got.Component.Kind == tt.want.Component.Kind) {
				t.Errorf("got %v, want %v", got.Component.Kind, tt.want.Component.Kind)
			}
			if !(got.Component.Version == tt.want.Component.Version) {
				t.Errorf("got %v, want %v", got.Component.Version, tt.want.Component.Version)
			}
		})
	}
}
