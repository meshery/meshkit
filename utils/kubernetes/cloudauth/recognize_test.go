package cloudauth

import (
	"testing"

	"github.com/meshery/meshkit/errors"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestMatchAuthInfo(t *testing.T) {
	tests := []struct {
		name        string
		authInfo    *clientcmdapi.AuthInfo
		wantMatched bool
		wantCode    string
		check       func(t *testing.T, m MatchedConfig)
	}{
		{
			name:        "nil authInfo is unmatched",
			authInfo:    nil,
			wantMatched: false,
		},
		{
			name:        "empty authInfo is unmatched",
			authInfo:    &clientcmdapi.AuthInfo{},
			wantMatched: false,
		},
		{
			name: "aws eks get-token standard flags",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws",
					Args: []string{
						"eks", "get-token",
						"--cluster-name", "prod-eks",
						"--region", "us-west-2",
						"--role-arn", "arn:aws:iam::123456789012:role/eks-admin",
						"--profile", "eks-prod",
					},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderEKS {
					t.Fatalf("Provider = %q, want %q", m.Provider, ProviderEKS)
				}
				if m.ClusterName != "prod-eks" {
					t.Fatalf("ClusterName = %q, want prod-eks", m.ClusterName)
				}
				if m.Region != "us-west-2" {
					t.Fatalf("Region = %q, want us-west-2", m.Region)
				}
				if m.RoleARN != "arn:aws:iam::123456789012:role/eks-admin" {
					t.Fatalf("RoleARN = %q, unexpected", m.RoleARN)
				}
				if m.Profile != "eks-prod" {
					t.Fatalf("Profile = %q, want eks-prod", m.Profile)
				}
			},
		},
		{
			name: "aws global flags before subcommand and equals syntax",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: `C:\Program Files\Amazon\AWSCLIV2\aws.exe`,
					Args: []string{
						"--profile=staging",
						"-r", "eu-west-1",
						"--endpoint-url=https://sts.eu-west-1.amazonaws.com",
						"eks", "get-token",
						"--cluster-name=staging-cluster",
					},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderEKS || m.Command != "aws" {
					t.Fatalf("got Provider=%q Command=%q", m.Provider, m.Command)
				}
				if m.ClusterName != "staging-cluster" || m.Region != "eu-west-1" || m.Profile != "staging" {
					t.Fatalf("unexpected parsed fields: %+v", m)
				}
				if m.Endpoint != "https://sts.eu-west-1.amazonaws.com" {
					t.Fatalf("Endpoint = %q", m.Endpoint)
				}
			},
		},
		{
			name: "aws eks get-token uses exec env fallback",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "/usr/local/bin/aws",
					Args:    []string{"eks", "get-token", "--cluster-name", "env-eks"},
					Env: []clientcmdapi.ExecEnvVar{
						{Name: "AWS_DEFAULT_REGION", Value: "ap-northeast-1"},
						{Name: "AWS_PROFILE", Value: "env-profile"},
					},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Region != "ap-northeast-1" || m.Profile != "env-profile" {
					t.Fatalf("got Region=%q Profile=%q", m.Region, m.Profile)
				}
			},
		},
		{
			name: "aws non-eks command is unmatched",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws",
					Args:    []string{"s3", "ls"},
				},
			},
			wantMatched: false,
		},
		{
			name: "aws eks get-token missing cluster name returns invalid config error",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws",
					Args:    []string{"eks", "get-token", "--region", "us-east-1"},
				},
			},
			wantMatched: true,
			wantCode:    ErrInvalidCloudExecConfigCode,
		},
		{
			name: "aws eks get-token with mfa flag rejected in v1",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws",
					Args:    []string{"eks", "get-token", "--cluster-name", "c1", "--mfa-serial", "arn:aws:iam::123:mfa/u"},
				},
			},
			wantMatched: true,
			wantCode:    ErrUnsupportedCloudAuthFlowCode,
		},
		{
			name: "aws eks get-token outposts --cluster-id recognized",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws",
					Args:    []string{"eks", "get-token", "--cluster-id", "outpost-123", "--region", "us-west-2"},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderEKS || m.ClusterName != "outpost-123" || m.Region != "us-west-2" {
					t.Fatalf("unexpected match: %+v", m)
				}
			},
		},
		{
			name: "aws-iam-authenticator token parsed cleanly including -s session short flag",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws-iam-authenticator",
					Args: []string{
						"token",
						"-i", "auth-cluster",
						"-r", "arn:aws:iam::123456789012:role/AuthRole",
						"--external-id", "ext-123",
						"-s", "meshery-short",
						"--region", "ap-south-1",
					},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderEKS || m.ClusterName != "auth-cluster" || m.Region != "ap-south-1" {
					t.Fatalf("unexpected match: %+v", m)
				}
				if m.RoleARN != "arn:aws:iam::123456789012:role/AuthRole" || m.ExternalID != "ext-123" || m.RoleSessionName != "meshery-short" {
					t.Fatalf("unexpected role fields: %+v", m)
				}
			},
		},
		{
			name: "aws-iam-authenticator missing cluster id errors",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "aws-iam-authenticator",
					Args:    []string{"token"},
				},
			},
			wantMatched: true,
			wantCode:    ErrInvalidCloudExecConfigCode,
		},
		{
			name: "gke-gcloud-auth-plugin recognized",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "/google-cloud-sdk/bin/gke-gcloud-auth-plugin",
					Args:    []string{"--use_application_default_credentials"},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderGKE || m.Command != "gke-gcloud-auth-plugin" {
					t.Fatalf("got Provider=%q Command=%q", m.Provider, m.Command)
				}
			},
		},
		{
			name: "gke-gcloud-auth-plugin impersonation rejected in v1",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "gke-gcloud-auth-plugin",
					Args:    []string{"--impersonate_service_account=sa@proj.iam.gserviceaccount.com"},
				},
			},
			wantMatched: true,
			wantCode:    ErrUnsupportedCloudAuthFlowCode,
		},
		{
			name: "gcloud config config-helper recognized",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "gcloud",
					Args:    []string{"config", "config-helper", "--format=json"},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderGKE || m.Command != "gcloud" {
					t.Fatalf("got Provider=%q Command=%q", m.Provider, m.Command)
				}
			},
		},
		{
			name: "gcloud auth print-access-token with global flags recognized",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "gcloud",
					Args:    []string{"--verbosity=warning", "--billing-project", "p1", "auth", "print-access-token"},
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderGKE || m.Command != "gcloud" {
					t.Fatalf("got Provider=%q Command=%q", m.Provider, m.Command)
				}
			},
		},
		{
			name: "gcloud impersonate-service-account rejected in v1",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "gcloud",
					Args:    []string{"--impersonate-service-account=sa@proj.iam.gserviceaccount.com", "auth", "print-access-token"},
				},
			},
			wantMatched: true,
			wantCode:    ErrUnsupportedCloudAuthFlowCode,
		},
		{
			name: "gcloud non-token subcommand left for fallback",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "gcloud",
					Args:    []string{"compute", "instances", "list"},
				},
			},
			wantMatched: false,
		},
		{
			name: "legacy gcp auth-provider recognized",
			authInfo: &clientcmdapi.AuthInfo{
				AuthProvider: &clientcmdapi.AuthProviderConfig{
					Name: "gcp",
				},
			},
			wantMatched: true,
			check: func(t *testing.T, m MatchedConfig) {
				t.Helper()
				if m.Provider != ProviderGKE || !m.LegacyProvider {
					t.Fatalf("got Provider=%q LegacyProvider=%v", m.Provider, m.LegacyProvider)
				}
			},
		},
		{
			name: "unknown exec plugin left untouched for #986 fallback",
			authInfo: &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "kubelogin",
					Args:    []string{"get-token"},
				},
			},
			wantMatched: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, matched, err := MatchAuthInfo(tc.authInfo)
			if matched != tc.wantMatched {
				t.Fatalf("matched = %v, want %v (err=%v)", matched, tc.wantMatched, err)
			}
			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("expected error with code %q, got nil", tc.wantCode)
				}
				if gotCode := errors.GetCode(err); gotCode != tc.wantCode {
					t.Fatalf("error code = %q, want %q (err=%v)", gotCode, tc.wantCode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}
