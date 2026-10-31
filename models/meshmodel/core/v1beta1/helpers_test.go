package v1beta1

import (
	"errors"
	"strings"
	"testing"

	"github.com/meshery/meshkit/utils/kubernetes"
	modelv1beta1 "github.com/meshery/schemas/models/v1beta1/model"
	"github.com/meshery/schemas/models/v1beta3/component"
)

func TestResolveSourceURI(t *testing.T) {
	tests := []struct {
		name        string
		comp        component.ComponentDefinition
		wantURI     string
		wantErr     bool
		errContains string
	}{
		{
			name: "component metadata takes precedence over model metadata",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": "git://github.com/meshery/component/main/chart.tgz",
					},
				},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: &modelv1beta1.ModelDefinition_Metadata{
						AdditionalProperties: map[string]interface{}{
							"source_uri": "git://github.com/meshery/model/main/chart.tgz",
						},
					},
				},
			},
			wantURI: "git://github.com/meshery/component/main/chart.tgz",
			wantErr: false,
		},
		{
			name: "fallback to model metadata when component source_uri is absent",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{},
				},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: &modelv1beta1.ModelDefinition_Metadata{
						AdditionalProperties: map[string]interface{}{
							"source_uri": "git://github.com/meshery/model/main/chart.tgz",
						},
					},
				},
			},
			wantURI: "git://github.com/meshery/model/main/chart.tgz",
			wantErr: false,
		},
		{
			name: "fallback to model metadata when component source_uri is empty string",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": "   ",
					},
				},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: &modelv1beta1.ModelDefinition_Metadata{
						AdditionalProperties: map[string]interface{}{
							"source_uri": "git://github.com/meshery/model/main/chart.tgz",
						},
					},
				},
			},
			wantURI: "git://github.com/meshery/model/main/chart.tgz",
			wantErr: false,
		},
		{
			name: "neither component nor model has source_uri",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{},
				},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: &modelv1beta1.ModelDefinition_Metadata{
						AdditionalProperties: map[string]interface{}{},
					},
				},
			},
			wantURI: "",
			wantErr: false,
		},
		{
			name: "nil AdditionalProperties on component and nil model",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{},
				Model:    nil,
			},
			wantURI: "",
			wantErr: false,
		},
		{
			name: "nil Model.Metadata",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: nil,
				},
			},
			wantURI: "",
			wantErr: false,
		},
		{
			name: "invalid component source_uri type",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": 12345,
					},
				},
			},
			wantURI:     "",
			wantErr:     true,
			errContains: "invalid component source_uri type",
		},
		{
			name: "invalid model source_uri type when component source_uri absent",
			comp: component.ComponentDefinition{
				Metadata: component.ComponentDefinition_Metadata{},
				Model: &modelv1beta1.ModelDefinition{
					Metadata: &modelv1beta1.ModelDefinition_Metadata{
						AdditionalProperties: map[string]interface{}{
							"source_uri": []string{"invalid"},
						},
					},
				},
			},
			wantURI:     "",
			wantErr:     true,
			errContains: "invalid model source_uri type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURI, err := resolveSourceURI(tt.comp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveSourceURI() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("resolveSourceURI() error = %v, want err containing %q", err, tt.errContains)
			}
			if gotURI != tt.wantURI {
				t.Fatalf("resolveSourceURI() = %q, want %q", gotURI, tt.wantURI)
			}
		})
	}
}

func TestResolveNamespace(t *testing.T) {
	tests := []struct {
		name     string
		comp     component.ComponentDefinition
		fallback string
		wantNS   string
	}{
		{
			name: "valid namespace in configuration",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "custom-ns",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "custom-ns",
		},
		{
			name: "trims whitespace in configured namespace",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "   spaced-ns   ",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "spaced-ns",
		},
		{
			name: "empty namespace falls back",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "   ",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "missing namespace key falls back",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"other": "value",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "non-string namespace falls back",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": 1234,
				},
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "nil configuration falls back",
			comp: component.ComponentDefinition{
				Configuration: nil,
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "invalid configured namespace falls back to valid fallback",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "INVALID_NS",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "configured namespace containing underscores falls back to valid fallback",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "invalid_namespace_with_underscores",
				},
			},
			fallback: "fallback-ns",
			wantNS:   "fallback-ns",
		},
		{
			name: "invalid configured namespace and invalid fallback falls back to default",
			comp: component.ComponentDefinition{
				Configuration: map[string]interface{}{
					"namespace": "INVALID_NS",
				},
			},
			fallback: "INVALID_FALLBACK",
			wantNS:   "default",
		},
		{
			name: "empty fallback falls back to default",
			comp: component.ComponentDefinition{
				Configuration: nil,
			},
			fallback: "",
			wantNS:   "default",
		},
		{
			name: "uppercase fallback falls back to default",
			comp: component.ComponentDefinition{
				Configuration: nil,
			},
			fallback: "UPPERCASE-FALLBACK",
			wantNS:   "default",
		},
		{
			name: "fallback containing underscores falls back to default",
			comp: component.ComponentDefinition{
				Configuration: nil,
			},
			fallback: "fallback_with_underscores",
			wantNS:   "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveNamespace(tt.comp, tt.fallback)
			if got != tt.wantNS {
				t.Fatalf("resolveNamespace() = %q, want %q", got, tt.wantNS)
			}
		})
	}
}

func TestClassifyGitHubChartURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURI      string
		wantURL     string
		wantInstall bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid git tar.gz archive",
			rawURI:      "git://github.com/meshery-extensions/wasm-filters/main/chart.tar.gz",
			wantURL:     "https://raw.githubusercontent.com/meshery-extensions/wasm-filters/main/chart.tar.gz",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "valid git tgz archive",
			rawURI:      "git://github.com/owner/repo/v1.0.0/operator.tgz",
			wantURL:     "https://raw.githubusercontent.com/owner/repo/v1.0.0/operator.tgz",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "mixed-case extension TAR.GZ",
			rawURI:      "git://github.com/owner/repo/v1.0.0/chart.TAR.GZ",
			wantURL:     "https://raw.githubusercontent.com/owner/repo/v1.0.0/chart.TAR.GZ",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "mixed-case extension TGZ",
			rawURI:      "git://github.com/owner/repo/v1.0.0/chart.TGZ",
			wantURL:     "https://raw.githubusercontent.com/owner/repo/v1.0.0/chart.TGZ",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "git URL with query string and fragment strips them from raw URL",
			rawURI:      "git://github.com/owner/repo/main/chart.tgz?token=secret#section",
			wantURL:     "https://raw.githubusercontent.com/owner/repo/main/chart.tgz",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "direct HTTPS tgz archive",
			rawURI:      "https://github.com/owner/repo/releases/download/v1.0.0/operator-v1.0.0.tgz",
			wantURL:     "https://github.com/owner/repo/releases/download/v1.0.0/operator-v1.0.0.tgz",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "direct HTTPS tar.gz archive",
			rawURI:      "https://example.com/charts/mychart.tar.gz",
			wantURL:     "https://example.com/charts/mychart.tar.gz",
			wantInstall: true,
			wantErr:     false,
		},
		{
			name:        "direct HTTP tgz archive returns error (insecure scheme rejected)",
			rawURI:      "http://example.com/charts/mychart.tgz",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "insecure HTTP scheme",
		},
		{
			name:        "direct HTTP tar.gz archive returns error (insecure scheme rejected)",
			rawURI:      "http://example.com/charts/mychart.tar.gz",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "insecure HTTP scheme",
		},
		{
			name:        "non-archive HTTP raw yaml returns error (insecure scheme rejected)",
			rawURI:      "http://example.com/crds/alloydb.yaml",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "insecure HTTP scheme",
		},
		{
			name:        "non-archive HTTP directory returns error (insecure scheme rejected)",
			rawURI:      "http://example.com/charts/",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "insecure HTTP scheme",
		},
		{
			name:        "non-archive HTTPS raw yaml skipped without error",
			rawURI:      "https://raw.githubusercontent.com/GoogleCloudPlatform/k8s-config-connector/master/crds/alloydb.yaml",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "non-archive HTTPS json skipped without error",
			rawURI:      "https://example.com/spec/crd.json",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "git directory path skipped without error",
			rawURI:      "git://github.com/meshery/meshery/master/install/kubernetes/helm/meshery-operator/",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "git yaml file skipped without error",
			rawURI:      "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/alloydb.yaml",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "ordinary github repository URL skipped without error",
			rawURI:      "https://github.com/meshery/meshery",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "empty URI skipped without error",
			rawURI:      "",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "whitespace-only URI skipped without error",
			rawURI:      "   \t\n  ",
			wantURL:     "",
			wantInstall: false,
			wantErr:     false,
		},
		{
			name:        "incomplete git URI with fewer than 4 segments returns error",
			rawURI:      "git://github.com/owner/repo",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "expected 4 segments",
		},
		{
			name:        "non-github host under git scheme returns error",
			rawURI:      "git://gitlab.com/owner/repo/main/chart.tgz",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "only git://github.com/ is supported",
		},
		{
			name:        "unsupported scheme ftp returns error",
			rawURI:      "ftp://github.com/owner/repo/main/chart.tgz",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "unsupported scheme",
		},
		{
			name:        "unsupported scheme ssh returns error",
			rawURI:      "ssh://git@github.com/owner/repo/chart.tgz",
			wantURL:     "",
			wantInstall: false,
			wantErr:     true,
			errContains: "unsupported scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotInstall, err := classifyGitHubChartURL(tt.rawURI)
			if (err != nil) != tt.wantErr {
				t.Fatalf("classifyGitHubChartURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("classifyGitHubChartURL() error = %v, want err containing %q", err, tt.errContains)
			}
			if gotInstall != tt.wantInstall {
				t.Fatalf("classifyGitHubChartURL() shouldInstall = %v, want %v", gotInstall, tt.wantInstall)
			}
			if gotURL != tt.wantURL {
				t.Fatalf("classifyGitHubChartURL() normalizedURL = %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}

func TestArtifactHubHandleDependents(t *testing.T) {
	ah := ArtifactHub{}

	t.Run("missing source_uri returns observable summary and nil error", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
		}

		summary, err := ah.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		want := "no source_uri declared for TestComponent"
		if summary != want {
			t.Fatalf("summary = %q, want %q", summary, want)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("invalid source_uri type returns error and empty summary", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": 99999,
				},
			},
		}

		summary, err := ah.HandleDependents(comp, nil, true, false)
		if err == nil {
			t.Fatal("expected error for non-string source_uri, got nil")
		}
		if summary != "" {
			t.Fatalf("expected empty summary, got %q", summary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("namespace selection from configuration", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Configuration: map[string]interface{}{
				"namespace": "my-custom-ns",
			},
		}

		summary, err := ah.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "my-custom-ns" {
			t.Fatalf("captured namespace = %q, want %q", capturedCfg.Namespace, "my-custom-ns")
		}
		if summary != "Deployed dependent helm chart for TestComponent" {
			t.Fatalf("summary = %q, want %q", summary, "Deployed dependent helm chart for TestComponent")
		}
	})

	t.Run("namespace fallback to default when configuration missing or nil", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Configuration: nil,
		}

		summary, err := ah.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "default" {
			t.Fatalf("captured namespace = %q, want %q", capturedCfg.Namespace, "default")
		}
		if summary != "Deployed dependent helm chart for TestComponent" {
			t.Fatalf("summary = %q, want %q", summary, "Deployed dependent helm chart for TestComponent")
		}
	})

	t.Run("action mapping for deploy success", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/wasm.tgz",
				},
			},
		}

		summary, err := ah.HandleDependents(comp, nil, true, true)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Action != kubernetes.INSTALL {
			t.Fatalf("captured action = %v, want INSTALL", capturedCfg.Action)
		}
		if !capturedCfg.CreateNamespace {
			t.Fatal("expected CreateNamespace to be true")
		}
		if !capturedCfg.UpgradeIfInstalled {
			t.Fatal("expected UpgradeIfInstalled to be true")
		}
		wantSummary := "Deployed dependent helm chart for WasmPlugin"
		if summary != wantSummary {
			t.Fatalf("summary = %q, want %q", summary, wantSummary)
		}
	})

	t.Run("action mapping for deploy failure preserves typo spelling", func(t *testing.T) {
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		helmErr := errors.New("cluster connection timeout")
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			return helmErr
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/wasm.tgz",
				},
			},
		}

		summary, err := ah.HandleDependents(comp, nil, true, false)
		if !errors.Is(err, helmErr) {
			t.Fatalf("error = %v, want %v", err, helmErr)
		}
		wantSummary := "error deploying dependent helm chart for WasmPlugin, please procced with manual install or try again"
		if summary != wantSummary {
			t.Fatalf("summary = %q, want %q", summary, wantSummary)
		}
	})

	t.Run("action mapping for undeploy success", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/wasm.tgz",
				},
			},
		}

		summary, err := ah.HandleDependents(comp, nil, false, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Action != kubernetes.UNINSTALL {
			t.Fatalf("captured action = %v, want UNINSTALL", capturedCfg.Action)
		}
		wantSummary := "Undeployed dependent helm chart for WasmPlugin"
		if summary != wantSummary {
			t.Fatalf("summary = %q, want %q", summary, wantSummary)
		}
	})

	t.Run("action mapping for undeploy failure", func(t *testing.T) {
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		helmErr := errors.New("release not found")
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			return helmErr
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/wasm.tgz",
				},
			},
		}

		summary, err := ah.HandleDependents(comp, nil, false, false)
		if !errors.Is(err, helmErr) {
			t.Fatalf("error = %v, want %v", err, helmErr)
		}
		wantSummary := "error undeploying dependent helm chart for WasmPlugin, please proceed with manual uninstall or try again"
		if summary != wantSummary {
			t.Fatalf("summary = %q, want %q", summary, wantSummary)
		}
	})

	t.Run("component metadata takes precedence over model metadata", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/comp-chart.tgz",
				},
			},
			Model: &modelv1beta1.ModelDefinition{
				Metadata: &modelv1beta1.ModelDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": "https://example.com/model-chart.tgz",
					},
				},
			},
		}

		_, err := ah.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.URL != "https://example.com/comp-chart.tgz" {
			t.Fatalf("captured URL = %q, want component URL %q", capturedCfg.URL, "https://example.com/comp-chart.tgz")
		}
	})

	t.Run("model metadata fallback when component metadata source_uri missing", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "TestComponent",
			Metadata:    component.ComponentDefinition_Metadata{},
			Model: &modelv1beta1.ModelDefinition{
				Metadata: &modelv1beta1.ModelDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": "https://example.com/model-chart.tgz",
					},
				},
			},
		}

		_, err := ah.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.URL != "https://example.com/model-chart.tgz" {
			t.Fatalf("captured URL = %q, want model fallback URL %q", capturedCfg.URL, "https://example.com/model-chart.tgz")
		}
	})
}

func TestGitHubHandleDependents(t *testing.T) {
	gh := GitHub{}

	t.Run("missing source_uri does not invoke Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		want := "no source_uri declared for WasmPlugin"
		if summary != want {
			t.Fatalf("summary = %q, want %q", summary, want)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("HTTP chart archives return error and do not invoke Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "http://example.com/charts/wasm-plugin.tgz",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err == nil {
			t.Fatal("expected error for HTTP chart archive, got nil")
		}
		if !strings.Contains(err.Error(), "insecure HTTP scheme") {
			t.Fatalf("error = %v, want containing insecure HTTP scheme", err)
		}
		if summary != "" {
			t.Fatalf("expected empty summary on error before Helm dispatch, got %q", summary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called for insecure HTTP source_uri")
		}
	})

	t.Run("invalid source_uri type returns error without invoking Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": 12345,
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err == nil {
			t.Fatal("expected error for invalid source_uri type, got nil")
		}
		if summary != "" {
			t.Fatalf("expected empty summary on error, got %q", summary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("HTTPS chart archives are passed through unchanged", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://github.com/owner/repo/releases/download/v1.0.0/chart.tgz",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		wantURL := "https://github.com/owner/repo/releases/download/v1.0.0/chart.tgz"
		if capturedCfg.URL != wantURL {
			t.Fatalf("captured URL = %q, want %q", capturedCfg.URL, wantURL)
		}
		if summary != "Deployed dependent helm chart for WasmPlugin" {
			t.Fatalf("summary = %q, want %q", summary, "Deployed dependent helm chart for WasmPlugin")
		}
	})

	t.Run("valid four-segment GitHub git URIs are normalized correctly", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "git://github.com/meshery-extensions/wasm-filters/main/chart.tar.gz",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		wantURL := "https://raw.githubusercontent.com/meshery-extensions/wasm-filters/main/chart.tar.gz"
		if capturedCfg.URL != wantURL {
			t.Fatalf("captured URL = %q, want %q", capturedCfg.URL, wantURL)
		}
		if summary != "Deployed dependent helm chart for WasmPlugin" {
			t.Fatalf("summary = %q, want %q", summary, "Deployed dependent helm chart for WasmPlugin")
		}
	})

	t.Run("non-archive sources are skipped without invoking Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "AlloyDBBackup",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://raw.githubusercontent.com/GoogleCloudPlatform/k8s-config-connector/master/crds/alloydb.yaml",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		wantSummary := "No Helm operator dependency found for AlloyDBBackup; skipped"
		if summary != wantSummary {
			t.Fatalf("summary = %q, want %q", summary, wantSummary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("unsupported Git hosts return error without invoking Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "git://gitlab.com/owner/repo/main/chart.tgz",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err == nil {
			t.Fatal("expected error for non-github host, got nil")
		}
		if summary != "" {
			t.Fatalf("expected empty summary on error, got %q", summary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("unsupported schemes return error without invoking Helm", func(t *testing.T) {
		called := false
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			called = true
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "ftp://github.com/owner/repo/main/chart.tgz",
				},
			},
		}

		summary, err := gh.HandleDependents(comp, nil, true, false)
		if err == nil {
			t.Fatal("expected error for unsupported scheme, got nil")
		}
		if summary != "" {
			t.Fatalf("expected empty summary on error, got %q", summary)
		}
		if called {
			t.Fatal("applyHelmChartFunc should not have been called")
		}
	})

	t.Run("namespace selection and fallback handling", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		// 1. Configured namespace takes precedence
		comp1 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Configuration: map[string]interface{}{
				"namespace": "configured-namespace",
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "model-namespace",
			},
		}
		_, err := gh.HandleDependents(comp1, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "configured-namespace" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "configured-namespace")
		}

		// 2. Model name fallback when configuration is nil
		comp2 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "model-namespace",
			},
		}
		_, err = gh.HandleDependents(comp2, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "model-namespace" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "model-namespace")
		}

		// 3. Default fallback when both configuration and model are nil
		comp3 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
		}
		_, err = gh.HandleDependents(comp3, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "default" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "default")
		}

		// 4. Uppercase model name falls back to default
		comp4 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "UPPERCASE-MODEL",
			},
		}
		_, err = gh.HandleDependents(comp4, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "default" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "default")
		}

		// 5. Model name containing underscores falls back to default
		comp5 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "model_with_underscores",
			},
		}
		_, err = gh.HandleDependents(comp5, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "default" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "default")
		}

		// 6. Empty model name falls back to default
		comp6 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "",
			},
		}
		_, err = gh.HandleDependents(comp6, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "default" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "default")
		}

		// 7. Invalid configured namespace falls back to valid model name
		comp7 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Configuration: map[string]interface{}{
				"namespace": "INVALID_NAMESPACE_NAME",
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "valid-model-ns",
			},
		}
		_, err = gh.HandleDependents(comp7, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "valid-model-ns" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "valid-model-ns")
		}

		// 8. Valid configured namespace preserved even when model name is invalid
		comp8 := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
			Configuration: map[string]interface{}{
				"namespace": "valid-configured-ns",
			},
			Model: &modelv1beta1.ModelDefinition{
				Name: "UPPERCASE_INVALID_MODEL",
			},
		}
		_, err = gh.HandleDependents(comp8, nil, true, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Namespace != "valid-configured-ns" {
			t.Fatalf("namespace = %q, want %q", capturedCfg.Namespace, "valid-configured-ns")
		}
	})

	t.Run("deploy and undeploy map to correct Helm actions", func(t *testing.T) {
		var capturedCfg kubernetes.ApplyHelmChartConfig
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			capturedCfg = cfg
			return nil
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
		}

		// Deploy
		summary, err := gh.HandleDependents(comp, nil, true, true)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Action != kubernetes.INSTALL {
			t.Fatalf("Action = %v, want INSTALL", capturedCfg.Action)
		}
		if !capturedCfg.UpgradeIfInstalled {
			t.Fatal("expected UpgradeIfInstalled to be true")
		}
		if summary != "Deployed dependent helm chart for WasmPlugin" {
			t.Fatalf("summary = %q, want %q", summary, "Deployed dependent helm chart for WasmPlugin")
		}

		// Undeploy
		summary, err = gh.HandleDependents(comp, nil, false, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if capturedCfg.Action != kubernetes.UNINSTALL {
			t.Fatalf("Action = %v, want UNINSTALL", capturedCfg.Action)
		}
		if summary != "Undeployed dependent helm chart for WasmPlugin" {
			t.Fatalf("summary = %q, want %q", summary, "Undeployed dependent helm chart for WasmPlugin")
		}
	})

	t.Run("Helm failures are propagated and summarized", func(t *testing.T) {
		orig := applyHelmChartFunc
		t.Cleanup(func() { applyHelmChartFunc = orig })
		helmErr := errors.New("failed to install release: timeout")
		applyHelmChartFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
			return helmErr
		}

		comp := component.ComponentDefinition{
			DisplayName: "WasmPlugin",
			Metadata: component.ComponentDefinition_Metadata{
				AdditionalProperties: map[string]interface{}{
					"source_uri": "https://example.com/chart.tgz",
				},
			},
		}

		// Deploy failure
		summary, err := gh.HandleDependents(comp, nil, true, false)
		if !errors.Is(err, helmErr) {
			t.Fatalf("err = %v, want %v", err, helmErr)
		}
		wantDeployErrSummary := "error deploying dependent helm chart for WasmPlugin, please proceed with manual install or try again: failed to install release: timeout"
		if summary != wantDeployErrSummary {
			t.Fatalf("summary = %q, want %q", summary, wantDeployErrSummary)
		}

		// Undeploy failure
		summary, err = gh.HandleDependents(comp, nil, false, false)
		if !errors.Is(err, helmErr) {
			t.Fatalf("err = %v, want %v", err, helmErr)
		}
		wantUndeployErrSummary := "error undeploying dependent helm chart for WasmPlugin, please proceed with manual uninstall or try again: failed to install release: timeout"
		if summary != wantUndeployErrSummary {
			t.Fatalf("summary = %q, want %q", summary, wantUndeployErrSummary)
		}
	})
}
