package cloudauth

import (
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestRecognize(t *testing.T) {
	tests := []struct {
		name   string
		exec   *clientcmdapi.ExecConfig
		want   Spec
		wantOK bool
	}{
		{
			name: "aws eks get-token with separate flag values",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "demo", "--output", "json"},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2"}},
			wantOK: true,
		},
		{
			name: "aws eks get-token with equals flag values, role and profile",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name=demo", "--region=eu-west-1", "--role-arn=arn:aws:iam::111122223333:role/eks", "--profile=prod"},
			},
			want: Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{
				ClusterID: "demo", Region: "eu-west-1", Profile: "prod",
				RoleARN: "arn:aws:iam::111122223333:role/eks", RoleSessionName: awsCLISessionName,
			}},
			wantOK: true,
		},
		{
			name: "aws eks get-token region and profile from exec env",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo"},
				Env: []clientcmdapi.ExecEnvVar{
					{Name: "AWS_PROFILE", Value: "staging"},
					{Name: "AWS_REGION", Value: "ap-south-1"},
					{Name: "AWS_DEFAULT_REGION", Value: "us-east-2"},
				},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo", Region: "ap-south-1", Profile: "staging"}},
			wantOK: true,
		},
		{
			name: "aws eks get-token falls back to AWS_DEFAULT_REGION",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo"},
				Env:     []clientcmdapi.ExecEnvVar{{Name: "AWS_DEFAULT_REGION", Value: "us-east-2"}},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo", Region: "us-east-2"}},
			wantOK: true,
		},
		{
			name: "aws eks get-token region flag wins over exec env",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo", "--region", "us-west-2"},
				Env:     []clientcmdapi.ExecEnvVar{{Name: "AWS_REGION", Value: "ap-south-1"}},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2"}},
			wantOK: true,
		},
		{
			name: "aws eks get-token ignores credentials and shared files in exec env",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo"},
				Env: []clientcmdapi.ExecEnvVar{
					{Name: "AWS_ACCESS_KEY_ID", Value: "AKID"},
					{Name: "AWS_SECRET_ACCESS_KEY", Value: "SECRET"},
					{Name: "AWS_SESSION_TOKEN", Value: "SESSION"},
					{Name: "AWS_CONFIG_FILE", Value: "/etc/aws/config"},
					{Name: "AWS_SHARED_CREDENTIALS_FILE", Value: "/etc/aws/credentials"},
				},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "demo"}},
			wantOK: true,
		},
		{
			name: "aws eks get-token for an Outposts local cluster signs the cluster ID",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo", "--cluster-id", "0123abcd"},
			},
			want:   Spec{Provider: ProviderEKS, Command: "aws", EKS: EKSConfig{ClusterID: "0123abcd"}},
			wantOK: true,
		},
		{
			name: "absolute path to aws",
			exec: &clientcmdapi.ExecConfig{
				Command: "/usr/local/bin/aws",
				Args:    []string{"eks", "get-token", "--cluster-name", "demo"},
			},
			want:   Spec{Provider: ProviderEKS, Command: "/usr/local/bin/aws", EKS: EKSConfig{ClusterID: "demo"}},
			wantOK: true,
		},
		{
			name: "windows path to aws.exe",
			exec: &clientcmdapi.ExecConfig{
				Command: `C:\Program Files\Amazon\AWSCLIV2\AWS.EXE`,
				Args:    []string{"eks", "get-token", "--cluster-name", "demo"},
			},
			want:   Spec{Provider: ProviderEKS, Command: `C:\Program Files\Amazon\AWSCLIV2\AWS.EXE`, EKS: EKSConfig{ClusterID: "demo"}},
			wantOK: true,
		},
		{
			name: "aws-iam-authenticator token with short flags",
			exec: &clientcmdapi.ExecConfig{
				Command: "aws-iam-authenticator",
				Args:    []string{"token", "-i", "demo", "-r", "arn:aws:iam::111122223333:role/eks", "--region", "us-west-2"},
			},
			want: Spec{Provider: ProviderEKS, Command: "aws-iam-authenticator", EKS: EKSConfig{
				ClusterID: "demo", Region: "us-west-2", RoleARN: "arn:aws:iam::111122223333:role/eks",
			}},
			wantOK: true,
		},
		{
			name: "aws-iam-authenticator.exe token with long flags",
			exec: &clientcmdapi.ExecConfig{
				Command: `C:\bin\aws-iam-authenticator.exe`,
				Args:    []string{"token", "--cluster-id=demo", "--role=arn:aws:iam::111122223333:role/eks", "-s", "meshery", "-e", "ext-id"},
				Env:     []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "prod"}},
			},
			want: Spec{Provider: ProviderEKS, Command: `C:\bin\aws-iam-authenticator.exe`, EKS: EKSConfig{
				ClusterID: "demo", Profile: "prod", RoleARN: "arn:aws:iam::111122223333:role/eks",
				RoleSessionName: "meshery", ExternalID: "ext-id",
			}},
			wantOK: true,
		},
		{
			name:   "gke-gcloud-auth-plugin",
			exec:   &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"},
			want:   Spec{Provider: ProviderGKE, Command: "gke-gcloud-auth-plugin"},
			wantOK: true,
		},
		{
			name:   "absolute path to gke-gcloud-auth-plugin.exe",
			exec:   &clientcmdapi.ExecConfig{Command: `C:\google-cloud-sdk\bin\gke-gcloud-auth-plugin.exe`},
			want:   Spec{Provider: ProviderGKE, Command: `C:\google-cloud-sdk\bin\gke-gcloud-auth-plugin.exe`},
			wantOK: true,
		},
		{
			name: "gcloud config config-helper",
			exec: &clientcmdapi.ExecConfig{
				Command: "/opt/google-cloud-sdk/bin/gcloud",
				Args:    []string{"config", "config-helper", "--format=json"},
			},
			want:   Spec{Provider: ProviderGKE, Command: "/opt/google-cloud-sdk/bin/gcloud"},
			wantOK: true,
		},
		{
			name: "gcloud.cmd on windows",
			exec: &clientcmdapi.ExecConfig{
				Command: `C:\google-cloud-sdk\bin\gcloud.cmd`,
				Args:    []string{"config", "config-helper"},
			},
			want: Spec{Provider: ProviderGKE, Command: `C:\google-cloud-sdk\bin\gcloud.cmd`},
			// gcloud ships as a .cmd script on Windows.
			wantOK: true,
		},
		{
			name: "gcloud auth print-identity-token is left to client-go",
			exec: &clientcmdapi.ExecConfig{
				Command: "gcloud",
				Args:    []string{"auth", "print-identity-token", "--audiences=https://iap.example.com"},
			},
		},
		{
			name: "gcloud without arguments is left to client-go",
			exec: &clientcmdapi.ExecConfig{Command: "/usr/bin/gcloud"},
		},
		{
			name: "aws without eks get-token",
			exec: &clientcmdapi.ExecConfig{Command: "aws", Args: []string{"sts", "get-caller-identity"}},
		},
		{
			name: "aws eks get-token without a cluster name",
			exec: &clientcmdapi.ExecConfig{Command: "aws", Args: []string{"eks", "get-token"}},
		},
		{
			name: "aws-iam-authenticator without token subcommand",
			exec: &clientcmdapi.ExecConfig{Command: "aws-iam-authenticator", Args: []string{"verify", "-i", "demo"}},
		},
		{
			name: "aws-iam-authenticator token without cluster ID",
			exec: &clientcmdapi.ExecConfig{Command: "aws-iam-authenticator", Args: []string{"token"}},
		},
		{
			name: "kubelogin",
			exec: &clientcmdapi.ExecConfig{Command: "kubelogin", Args: []string{"get-token", "--server-id", "abc"}},
		},
		{
			name: "custom plugin by absolute path",
			exec: &clientcmdapi.ExecConfig{Command: "/usr/local/bin/custom", Args: []string{"eks", "get-token", "--cluster-name", "demo"}},
		},
		{
			name: "command that only contains a known name",
			exec: &clientcmdapi.ExecConfig{Command: "/opt/aws/bin/aws-vault", Args: []string{"exec", "prod"}},
		},
		{
			name: "nil exec config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Recognize(tt.exec)
			if ok != tt.wantOK {
				t.Fatalf("Recognize() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("Recognize() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSpecCacheKeySeparatesIdentities(t *testing.T) {
	base := Spec{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2"}}
	variants := []Spec{
		{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "other", Region: "us-west-2"}},
		{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "demo", Region: "eu-west-1"}},
		{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2", Profile: "prod"}},
		{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2", RoleARN: "arn:aws:iam::1:role/x"}},
		{Provider: ProviderEKS, EKS: EKSConfig{ClusterID: "demo", Region: "us-west-2", ExternalID: "ext"}},
		{Provider: ProviderGKE},
	}
	for _, v := range variants {
		if v.cacheKey() == base.cacheKey() {
			t.Fatalf("cacheKey(%#v) collides with cacheKey(%#v)", v, base)
		}
	}

	sameIdentity := base
	sameIdentity.Command = "/usr/local/bin/aws"
	if sameIdentity.cacheKey() != base.cacheKey() {
		t.Fatal("cacheKey depends on the command path, want it to depend only on the cluster identity")
	}
}
