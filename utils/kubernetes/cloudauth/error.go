package cloudauth

import (
	stderrors "errors"
	"fmt"
	"time"

	"github.com/meshery/meshkit/errors"
)

const (
	ErrInProcessCloudAuthUnavailableCode    = "meshkit-11341"
	ErrAWSCredentialsUnavailableCode        = "meshkit-11342"
	ErrEKSRegionUnresolvedCode              = "meshkit-11343"
	ErrMintEKSTokenCode                     = "meshkit-11344"
	ErrGCPCredentialsUnavailableCode        = "meshkit-11345"
	ErrGCPServiceAccountKeyCode             = "meshkit-11346"
	ErrUnsupportedCloudProviderCode         = "meshkit-11347"
	ErrInProcessCloudAuthFallbackCode       = "meshkit-11348"
	ErrUntrustedClusterEndpointCode         = "meshkit-11349"
	ErrUntrustedClusterEndpointFallbackCode = "meshkit-11350"
	ErrCloudAuthTimeoutCode                 = "meshkit-11351"
	ErrInProcessGKEIPMintingCode            = "meshkit-11352"
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
			fmt.Sprintf("The kubeconfig authenticates to %s with the exec credential plugin %q, which is not installed on this host's PATH", provider, command),
			"In-process token minting requires cloud credentials in the Meshery process; a kubeconfig alone is not enough",
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

// ErrInProcessCloudAuthFallback is logged by Authenticator.Apply when a
// kubeconfig names a recognized cloud exec plugin, in-process token minting
// could not be set up, and the plugin binary is on PATH, so the exec stanza
// is left for client-go to run instead.
func ErrInProcessCloudAuthFallback(err error, provider Provider, command string) error {
	return errors.NewV2(ErrInProcessCloudAuthFallbackCode, errors.None,
		[]string{"Falling back to the exec credential plugin for the managed Kubernetes cluster"},
		[]string{
			fmt.Sprintf("In-process token minting for %s could not be set up, so the exec credential plugin %q found on PATH will authenticate instead", provider, command),
			err.Error(),
		},
		[]string{
			"The Meshery process has no usable cloud credentials for the cluster's account or project",
			"The cloud credentials are misconfigured, for example a mistyped role ARN or an expired Application Default Credentials login",
		},
		[]string{
			"No action is needed if the exec credential plugin is meant to authenticate",
			"To mint tokens in process, fix the cloud credentials available to the Meshery process; setup is retried periodically",
		},
		nil)
}

// ErrUntrustedClusterEndpoint is returned by Authenticator.Apply when a
// kubeconfig names a recognized cloud exec plugin but its cluster endpoint
// is not one the Meshery process's cloud credentials may be sent to, and the
// plugin binary is not on PATH, so there is no way to authenticate to the
// cluster.
func ErrUntrustedClusterEndpoint(err error, provider Provider, host string, command string) error {
	return errors.NewV2(ErrUntrustedClusterEndpointCode, errors.Alert,
		[]string{"Refusing to send cloud credentials to an unrecognized cluster endpoint"},
		[]string{
			fmt.Sprintf("The kubeconfig authenticates to %s with the exec credential plugin %q, which is not installed on this host's PATH, and its server %q is not an endpoint the Meshery process's own cloud credentials may be sent to", provider, command, host),
			err.Error(),
		},
		[]string{
			"The kubeconfig server is not an EKS or GKE control plane endpoint, for example a proxy, a load balancer, or a host controlled by whoever wrote the kubeconfig",
			"The kubeconfig disables TLS verification (insecure-skip-tls-verify), uses http, or routes the connection through proxy-url",
			"For EKS, the exec stanza's --region differs from the region in the endpoint host name",
			"For a GKE IP endpoint, the kubeconfig has no certificate-authority data, or the GKE API did not confirm that this IP and CA certificate belong to a cluster in a trusted project (the ADC project, GOOGLE_CLOUD_PROJECT or GOOGLE_PROJECT)",
		},
		[]string{
			"Use the cluster endpoint written by aws eks update-kubeconfig or gcloud container clusters get-credentials, with TLS verification enabled",
			"For a GKE IP endpoint, grant the Meshery process container.clusters.get or container.clusters.list on the cluster project, and set GOOGLE_CLOUD_PROJECT to that project unless it is the ADC project",
			"Alternatively install the exec credential plugin named in the kubeconfig on the Meshery host's PATH, so it authenticates instead",
			"To keep the exec plugin for a context, disable in-process cloud auth for that context's kubeconfig user; every context sharing that user switches with it",
			"If the Meshery server cannot hold cloud credentials, deploy the Meshery Operator into the cluster instead",
		},
		nil)
}

// ErrUntrustedClusterEndpointFallback is logged by Authenticator.Apply when a
// kubeconfig names a recognized cloud exec plugin, its cluster endpoint is
// not one the Meshery process's cloud credentials may be sent to, and the
// plugin binary is on PATH, so the exec stanza is left for client-go to run.
func ErrUntrustedClusterEndpointFallback(err error, provider Provider, host string, command string) error {
	return errors.NewV2(ErrUntrustedClusterEndpointFallbackCode, errors.None,
		[]string{"Using the exec credential plugin for an unrecognized cluster endpoint"},
		[]string{
			fmt.Sprintf("The server %q is not an endpoint the Meshery process's own %s credentials may be sent to, so the exec credential plugin %q found on PATH will authenticate instead of in-process token minting", host, provider, command),
			err.Error(),
		},
		[]string{
			"The kubeconfig server is not an EKS or GKE control plane endpoint, or the kubeconfig disables TLS verification, uses http, or sets proxy-url",
			"For a GKE IP endpoint, no trusted project (the ADC project, GOOGLE_CLOUD_PROJECT or GOOGLE_PROJECT) was set, or the GKE API was unreachable, denied container.clusters.get and container.clusters.list, listed clusters only partially, or returned no cluster with this IP and CA certificate in those projects",
		},
		[]string{
			"No action is needed if the exec credential plugin is meant to authenticate",
			"To mint tokens in process, point the kubeconfig at the cluster's own EKS or GKE endpoint with TLS verification enabled",
			"For a GKE IP endpoint, grant the Meshery process container.clusters.get or container.clusters.list on the cluster project, and set GOOGLE_CLOUD_PROJECT to that project unless it is the ADC project",
		},
		nil)
}

// ErrCloudAuthTimeout is returned when setting up in-process credentials or
// minting a token did not finish within its deadline. The attempt is
// abandoned and the next request starts a new one.
func ErrCloudAuthTimeout(err error, provider Provider, operation string, timeout time.Duration) error {
	return errors.NewV2(ErrCloudAuthTimeoutCode, errors.Alert,
		[]string{"Timed out obtaining cloud credentials for the managed Kubernetes cluster"},
		[]string{
			fmt.Sprintf("In-process token minting for %s gave up %s after %s", provider, operation, timeout),
			err.Error(),
		},
		[]string{
			"The cloud provider's token or credentials endpoint (AWS STS, Google OAuth2, or an instance metadata server) is unreachable or not responding",
			"A network proxy or firewall is blocking or stalling requests from the Meshery process to the cloud provider",
		},
		[]string{
			"Verify that the Meshery host can reach the cloud provider's credential endpoints",
			"Retry the request; every new request starts a fresh attempt",
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
			fmt.Sprintf("Unable to resolve AWS credentials to mint a token for EKS cluster %q", clusterID),
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
		[]string{fmt.Sprintf("No AWS region is configured for EKS cluster %q", clusterID)},
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
			fmt.Sprintf("Presigning the STS GetCallerIdentity request for EKS cluster %q failed", clusterID),
			err.Error(),
		},
		[]string{"The AWS credentials expired and could not be refreshed"},
		[]string{"Verify that the AWS credentials available to the Meshery process are still valid"},
		nil)
}

// ErrGCPCredentialsUnavailable is returned when Google Application Default
// Credentials cannot be found, are of an unsupported kind, or cannot produce
// a token, for minting a GKE token.
func ErrGCPCredentialsUnavailable(err error) error {
	return errors.NewV2(ErrGCPCredentialsUnavailableCode, errors.Alert,
		[]string{"Google Cloud credentials are not available for GKE token minting"},
		[]string{
			"Unable to obtain a token from Google Application Default Credentials to mint a GKE token",
			err.Error(),
		},
		[]string{
			"GOOGLE_APPLICATION_CREDENTIALS is unset, no gcloud application-default credentials file exists, and no GCE or GKE metadata server is reachable",
			"The Application Default Credentials were found but token exchange failed, for example because the refresh token was revoked or expired",
			"The credentials file is not a service_account key or an authorized_user (gcloud application-default login) file; external_account, impersonated_service_account and other types are not supported in process",
			"The credentials file names a token_uri other than https://oauth2.googleapis.com/token",
			"On a metadata server, the host's access scopes do not include cloud-platform and userinfo.email",
		},
		[]string{
			"Set GOOGLE_APPLICATION_CREDENTIALS to a service-account key file, run on GKE with Workload Identity, or run gcloud auth application-default login",
			"On a Compute Engine host, enable the cloud-platform and userinfo.email access scopes",
		},
		nil)
}

// ErrGCPServiceAccountKey is returned when service-account key JSON passed
// to NewGKETokenSourceFromJSON cannot be used.
func ErrGCPServiceAccountKey(err error) error {
	return errors.NewV2(ErrGCPServiceAccountKeyCode, errors.Alert,
		[]string{"Unusable Google Cloud service-account key"},
		[]string{err.Error()},
		[]string{
			"The supplied JSON is malformed or is not a service_account key",
			"The key names a token_uri other than https://oauth2.googleapis.com/token",
			"Google rejected the key, for example because it was deleted or disabled, or the token endpoint could not be reached",
		},
		[]string{"Supply a current JSON key file downloaded for a Google Cloud service account, unmodified"},
		nil)
}

// ErrInProcessGKEIPMinting is logged by Authenticator when in-process
// minting is applied to a GKE IP endpoint that the Container API confirmed.
// It is logged once per setup, including the setup that follows a 401.
func ErrInProcessGKEIPMinting(ip, project, location, name string) error {
	return errors.NewV2(ErrInProcessGKEIPMintingCode, errors.None,
		[]string{"In-process GKE token minting is active for an IP endpoint"},
		[]string{
			fmt.Sprintf("In-process GKE token minting is active for IP endpoint %s (cluster %s/%s/%s, verified with the GKE API in a trusted project). Set MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true or disable in-process cloud auth for this context's kubeconfig user to use the exec plugin instead.", ip, project, location, name),
		},
		[]string{
			"The kubeconfig names a GKE control plane by IP address, and the GKE API confirmed that IP and CA certificate belong to a cluster in the ADC project, GOOGLE_CLOUD_PROJECT or GOOGLE_PROJECT",
		},
		[]string{
			"No action is needed if this cluster should use in-process token minting",
			"To use the exec plugin for this context instead, disable in-process cloud auth for that context's kubeconfig user (every context sharing that user switches with it), or set MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true for the whole process",
		},
		nil)
}

// ErrUnsupportedCloudProvider is returned by the default token source
// factory for a Spec whose provider it does not implement.
func ErrUnsupportedCloudProvider(provider Provider) error {
	return errors.NewV2(ErrUnsupportedCloudProviderCode, errors.Alert,
		[]string{"Unsupported cloud provider for in-process token minting"},
		[]string{fmt.Sprintf("In-process token minting does not support provider %q", provider)},
		[]string{"The credential plugin specification was constructed with an unknown provider"},
		[]string{"Use a Spec returned by Recognize"},
		nil)
}
