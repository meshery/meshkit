package cloudauth

import (
	stderrors "errors"
	"fmt"

	"github.com/meshery/meshkit/errors"
)

const (
	ErrInProcessCloudAuthUnavailableCode = "meshkit-11341"
	ErrAWSCredentialsUnavailableCode     = "meshkit-11342"
	ErrEKSRegionUnresolvedCode           = "meshkit-11343"
	ErrMintEKSTokenCode                  = "meshkit-11344"
	ErrGCPCredentialsUnavailableCode     = "meshkit-11345"
	ErrGCPServiceAccountKeyCode          = "meshkit-11346"
	ErrUnsupportedCloudProviderCode      = "meshkit-11347"
)

// errNoCredentialProvider is the cause reported when the AWS SDK returns a
// config without any credential provider at all.
var errNoCredentialProvider = stderrors.New("the AWS SDK resolved no credential provider")

// ErrInProcessCloudAuthUnavailable is returned by Authenticator.Apply when a
// kubeconfig names a recognized cloud exec plugin, in-process token minting
// could not obtain cloud credentials, and the plugin binary is not on PATH
// either, so there is no way to authenticate to the cluster.
func ErrInProcessCloudAuthUnavailable(err error, provider Provider, command string) error {
	return errors.NewV2(ErrInProcessCloudAuthUnavailableCode, errors.Alert,
		[]string{"Unable to authenticate to the managed Kubernetes cluster"},
		[]string{
			fmt.Sprintf("The kubeconfig authenticates to %s with the exec credential plugin %q, which is not installed on this host's PATH.", provider, command),
			"In-process token minting requires cloud credentials in the Meshery process; a kubeconfig alone is not enough.",
			err.Error(),
		},
		[]string{
			"The Meshery process has no cloud credentials for the cluster's account or project",
			"The exec credential plugin named in the kubeconfig is not installed where Meshery runs",
		},
		[]string{
			"For EKS, give the Meshery process AWS credentials: environment variables, a shared config profile, IAM Roles for Service Accounts, or an EC2/ECS instance role",
			"For GKE, give the Meshery process Application Default Credentials: GOOGLE_APPLICATION_CREDENTIALS pointing at a service-account key, or Workload Identity",
			"Alternatively install the exec credential plugin (the AWS CLI, aws-iam-authenticator, or gke-gcloud-auth-plugin) on the Meshery host's PATH",
			"If the Meshery server cannot hold cloud credentials, deploy the Meshery Operator into the cluster instead",
		},
		nil)
}

// ErrAWSCredentialsUnavailable is returned when the AWS SDK default
// credential chain (and any requested role assumption) yields no usable
// credentials for minting an EKS token.
func ErrAWSCredentialsUnavailable(err error, clusterID string) error {
	return errors.NewV2(ErrAWSCredentialsUnavailableCode, errors.Alert,
		[]string{"AWS credentials are not available for EKS token minting"},
		[]string{
			fmt.Sprintf("Unable to resolve AWS credentials to mint a token for EKS cluster %q.", clusterID),
			err.Error(),
		},
		[]string{
			"No AWS credentials are configured in the environment, shared config files, web identity (IRSA), or instance metadata",
			"The AWS profile named in the kubeconfig does not exist where Meshery runs",
			"The role named with --role-arn or -r cannot be assumed with the available credentials",
			"The profile uses an AWS IAM Identity Center (SSO) session that has expired, or an assume-role that requires MFA, neither of which is supported in process",
		},
		[]string{
			"Configure AWS credentials for the Meshery process (for example AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, AWS_PROFILE, IRSA, or an instance role)",
			"Verify that the credentials are allowed to assume the role referenced by the kubeconfig",
		},
		nil)
}

// ErrEKSRegionUnresolved is returned when neither the kubeconfig exec
// stanza nor the AWS configuration available to the process names a region.
func ErrEKSRegionUnresolved(clusterID string) error {
	return errors.NewV2(ErrEKSRegionUnresolvedCode, errors.Alert,
		[]string{"AWS region is not configured for EKS token minting"},
		[]string{fmt.Sprintf("No AWS region is configured for EKS cluster %q.", clusterID)},
		[]string{"The exec stanza has no --region flag, no AWS_REGION or AWS_DEFAULT_REGION env entry, and the process environment and shared config name no region either"},
		[]string{"Add --region to the exec stanza arguments, or set AWS_REGION for the Meshery process"},
		nil)
}

// ErrMintEKSToken is returned when presigning the STS GetCallerIdentity
// request that forms an EKS token fails, typically because the cached AWS
// credentials could not be refreshed.
func ErrMintEKSToken(err error, clusterID string) error {
	return errors.NewV2(ErrMintEKSTokenCode, errors.Alert,
		[]string{"Unable to mint EKS bearer token"},
		[]string{
			fmt.Sprintf("Presigning the STS GetCallerIdentity request for EKS cluster %q failed.", clusterID),
			err.Error(),
		},
		[]string{"The AWS credentials expired and could not be refreshed"},
		[]string{"Verify that the AWS credentials available to the Meshery process are still valid"},
		nil)
}

// ErrGCPCredentialsUnavailable is returned when Google Application Default
// Credentials cannot be found, or cannot produce a token, for minting a GKE
// token.
func ErrGCPCredentialsUnavailable(err error) error {
	return errors.NewV2(ErrGCPCredentialsUnavailableCode, errors.Alert,
		[]string{"Google Cloud credentials are not available for GKE token minting"},
		[]string{
			"Unable to obtain a token from Google Application Default Credentials to mint a GKE token.",
			err.Error(),
		},
		[]string{"GOOGLE_APPLICATION_CREDENTIALS is unset, no gcloud application-default credentials file exists, and no GCE or GKE metadata server is reachable", "The Application Default Credentials were found but token exchange failed, for example because the refresh token was revoked or expired"},
		[]string{"Set GOOGLE_APPLICATION_CREDENTIALS to a service-account key file, run on GKE with Workload Identity, or run gcloud auth application-default login"},
		nil)
}

// ErrGCPServiceAccountKey is returned when service-account key JSON passed
// to NewGKETokenSourceFromJSON cannot be used.
func ErrGCPServiceAccountKey(err error) error {
	return errors.NewV2(ErrGCPServiceAccountKeyCode, errors.Alert,
		[]string{"Invalid Google Cloud service-account key"},
		[]string{err.Error()},
		[]string{"The supplied JSON is malformed or is not a service_account key"},
		[]string{"Supply the JSON key file downloaded for a Google Cloud service account"},
		nil)
}

// ErrUnsupportedCloudProvider is returned by the default token source
// factory for a Spec whose provider it does not implement.
func ErrUnsupportedCloudProvider(provider Provider) error {
	return errors.NewV2(ErrUnsupportedCloudProviderCode, errors.Alert,
		[]string{"Unsupported cloud provider for in-process token minting"},
		[]string{fmt.Sprintf("In-process token minting does not support provider %q.", provider)},
		[]string{"The credential plugin specification was constructed with an unknown provider"},
		[]string{"Use a Spec returned by Recognize"},
		nil)
}
