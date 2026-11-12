package cloudauth

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

const (
	// ErrInvalidCloudExecConfigCode indicates a recognized EKS or GKE kubeconfig
	// exec stanza is missing required parameters (such as the EKS cluster name).
	ErrInvalidCloudExecConfigCode = "meshkit-11341"

	// ErrAWSCredentialsCode indicates AWS credentials could not be resolved from
	// the configured credential chain.
	ErrAWSCredentialsCode = "meshkit-11342"

	// ErrEKSTokenMintCode indicates generating or presigning the EKS STS bearer
	// token failed.
	ErrEKSTokenMintCode = "meshkit-11343"

	// ErrGCPCredentialsCode indicates Google Cloud credentials could not be
	// resolved from Application Default Credentials or service-account JSON.
	ErrGCPCredentialsCode = "meshkit-11344"

	// ErrGKETokenMintCode indicates acquiring or refreshing a Google OAuth2
	// access token for GKE failed.
	ErrGKETokenMintCode = "meshkit-11345"

	// ErrUnsupportedCloudAuthFlowCode indicates the kubeconfig relies on an
	// authentication flow outside v1 support (such as interactive AWS SSO or MFA).
	ErrUnsupportedCloudAuthFlowCode = "meshkit-11346"

	// ErrEKSRegionUnresolvedCode indicates no AWS region was specified in the
	// kubeconfig exec stanza, environment, shared config, or explicit credentials.
	ErrEKSRegionUnresolvedCode = "meshkit-11347"
)

// ErrInvalidCloudExecConfig returns a structured error for malformed or
// incomplete EKS/GKE exec stanzas.
func ErrInvalidCloudExecConfig(err error) error {
	return errors.NewV2(
		ErrInvalidCloudExecConfigCode,
		errors.Alert,
		[]string{"Invalid cloud Kubernetes authentication configuration."},
		[]string{fmt.Sprintf("Failed to parse cloud exec authentication configuration: %v", err)},
		[]string{
			"The kubeconfig exec stanza for EKS or GKE is missing required arguments (such as --cluster-name or -i).",
		},
		[]string{
			"Verify the kubeconfig users[].user.exec configuration and ensure all required flags are present.",
		},
		nil,
	)
}

// ErrAWSCredentials returns a structured error when AWS credentials cannot be
// resolved for EKS in-process authentication.
func ErrAWSCredentials(err error) error {
	return errors.NewV2(
		ErrAWSCredentialsCode,
		errors.Alert,
		[]string{"AWS credentials are unavailable for EKS authentication."},
		[]string{fmt.Sprintf("Unable to resolve AWS credentials for in-process EKS token minting: %v", err)},
		[]string{
			"No valid AWS credentials were provided explicitly or found in environment variables, shared AWS config/credentials files, IRSA web identity, or EC2 IMDS.",
			"Meshery Server or CLI lacks access to the target AWS account or IAM role.",
		},
		[]string{
			"Configure AWS credentials (environment variables, shared credentials/config, IRSA, or an attached Meshery AWS credential) and verify permissions to assume any configured IAM role.",
		},
		nil,
	)
}

// ErrEKSTokenMint returns a structured error when presigning STS
// GetCallerIdentity fails.
func ErrEKSTokenMint(err error) error {
	return errors.NewV2(
		ErrEKSTokenMintCode,
		errors.Alert,
		[]string{"Failed to mint EKS authentication token."},
		[]string{fmt.Sprintf("Unable to presign AWS STS GetCallerIdentity request for EKS: %v", err)},
		[]string{
			"AWS region, STS endpoint, or IAM credentials could not be used to sign the STS GetCallerIdentity token request.",
		},
		[]string{
			"Check the AWS region and credentials configuration, then retry.",
		},
		nil,
	)
}

// ErrGCPCredentials returns a structured error when Google Cloud credentials
// cannot be loaded from ADC or service-account JSON.
func ErrGCPCredentials(err error) error {
	return errors.NewV2(
		ErrGCPCredentialsCode,
		errors.Alert,
		[]string{"Google Cloud credentials are unavailable for GKE authentication."},
		[]string{fmt.Sprintf("Unable to load Google Cloud credentials for in-process GKE token minting: %v", err)},
		[]string{
			"No valid GCP Application Default Credentials, Workload Identity metadata, or service-account JSON key was found.",
		},
		[]string{
			"Configure GOOGLE_APPLICATION_CREDENTIALS, Application Default Credentials, GKE Workload Identity, or attach a valid GCP service-account JSON credential.",
		},
		nil,
	)
}

// ErrGKETokenMint returns a structured error when OAuth2 token acquisition
// fails for GKE.
func ErrGKETokenMint(err error) error {
	return errors.NewV2(
		ErrGKETokenMintCode,
		errors.Alert,
		[]string{"Failed to mint GKE authentication token."},
		[]string{fmt.Sprintf("Unable to obtain Google OAuth2 access token for GKE: %v", err)},
		[]string{
			"The configured GCP service-account key or Application Default Credentials could not exchange for a valid OAuth2 access token.",
		},
		[]string{
			"Verify that the GCP credentials are active, unrevoked, and have network access to Google OAuth2 token endpoints.",
		},
		nil,
	)
}

// ErrUnsupportedCloudAuthFlow returns a structured error for authentication
// mechanisms excluded from v1 (such as AWS SSO and interactive MFA).
func ErrUnsupportedCloudAuthFlow(reason string) error {
	return errors.NewV2(
		ErrUnsupportedCloudAuthFlowCode,
		errors.Alert,
		[]string{"Cloud authentication flow is not supported in v1."},
		[]string{reason},
		[]string{
			"AWS SSO (IAM Identity Center), interactive MFA prompts, and interactive gcloud logins are unsupported for in-process token minting in v1.",
		},
		[]string{
			"Use non-interactive AWS IAM credentials, AssumeRole, IRSA, GCP service-account JSON, or Workload Identity; or use Meshery Operator inside the cluster.",
		},
		nil,
	)
}

// ErrEKSRegionUnresolved returns a structured error when no AWS region is
// configured anywhere for signing EKS STS requests.
func ErrEKSRegionUnresolved(clusterName string) error {
	return errors.NewV2(
		ErrEKSRegionUnresolvedCode,
		errors.Alert,
		[]string{"AWS region is unresolved for EKS authentication."},
		[]string{fmt.Sprintf("Unable to determine AWS region for in-process EKS authentication to cluster %q.", clusterName)},
		[]string{
			"No --region flag, AWS_REGION/AWS_DEFAULT_REGION environment variable, shared config profile region, or explicit AWS region was provided.",
		},
		[]string{
			"Specify --region in the kubeconfig exec arguments, set AWS_REGION, or supply Region on explicit AWS credentials.",
		},
		nil,
	)
}
