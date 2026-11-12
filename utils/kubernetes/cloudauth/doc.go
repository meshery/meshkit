// Package cloudauth mints short-lived Kubernetes bearer tokens for managed
// cloud clusters in process, using the cloud provider SDKs, instead of
// forking the exec credential plugin named in a kubeconfig user's exec
// stanza.
//
// The package is deliberately narrow: it is the only part of MeshKit that
// imports the AWS and Google Cloud SDKs, and utils/kubernetes depends on it
// only through Authenticator.Apply. A kubeconfig whose exec stanza is not a
// recognized cloud plugin is never touched, so the client-go exec loader
// keeps handling every other credential plugin (kubelogin, custom scripts,
// and so on) exactly as before.
//
// # What is intercepted
//
// A stanza is replaced with in-process minting only when all of these hold:
//
//   - In-process minting is enabled (see DisableEnvVar).
//   - The rest.Config carries no credentials client-go prefers over an exec
//     plugin: no bearer token or token file, no basic-auth username, and no
//     client certificate with key. Such configs are left exactly as
//     client-go would use them.
//   - Recognize accepts the stanza (below): the Meshery process's own
//     credentials reproduce the identity the plugin would use.
//   - The cluster endpoint is one the process's credentials may be sent to
//     (below).
//
// # Supported exec plugins
//
// Recognize classifies an exec stanza by the basename of its command, so
// absolute paths and Windows-style ".exe", ".cmd" and ".bat" suffixes are
// accepted:
//
//   - Amazon EKS: "aws eks get-token" (flags --cluster-name, --cluster-id,
//     --region, --role-arn, --profile, in both "--flag value" and
//     "--flag=value" forms) and "aws-iam-authenticator token" (flags
//     -i/--cluster-id, -r/--role, -s/--session-name, -e/--external-id,
//     --region). A stanza that passes --forward-session-name stays on the
//     plugin: it forwards the caller's STS session name when assuming a
//     role, which in-process minting cannot reproduce. A flag whose value
//     is missing or is itself a flag (as in
//     "--region --cluster-name demo") makes the stanza malformed, and it is
//     left to the plugin.
//   - Google Kubernetes Engine: "gke-gcloud-auth-plugin
//     --use_application_default_credentials" with no other argument and no
//     env entries. Without that flag the plugin uses the gcloud CLI's active
//     account, which Application Default Credentials cannot reproduce, so a
//     bare gke-gcloud-auth-plugin stanza, one with any other flag (for
//     example --impersonate_service_account), and every gcloud stanza
//     (including "gcloud config config-helper", with or without --account
//     or --configuration) stay on the plugin.
//
// # Endpoint binding
//
// A kubeconfig is often written by someone other than the operator of the
// Meshery process, and an in-process token carries the process's own cloud
// identity, so a token is only ever attached to a cluster endpoint of the
// matching cloud. The server must use https with TLS verification on
// (insecure-skip-tls-verify unset) and no kubeconfig proxy-url, and:
//
//   - EKS: the host must be an EKS API endpoint,
//     <id>.<cell>.<region>.eks.amazonaws.com, or .eks.amazonaws.com.cn in the
//     China partition, or the IPv6 form <id>.<region>.api.aws, or
//     .api.amazonwebservices.com.cn in China. When the stanza names no
//     region, the endpoint's region is used; when it names a different one,
//     the stanza is not intercepted. An EKS token is a presigned STS URL that
//     anyone holding it can replay for 15 minutes against the named cluster
//     or present to STS, which is why it must not reach any other host.
//   - GKE: the host must be an IP address (public or private endpoint) with
//     the cluster CA certificate in the kubeconfig, a DNS-based control plane
//     endpoint under .gke.goog, or Connect Gateway
//     (connectgateway.googleapis.com or <region>-connectgateway.googleapis.com).
//
// Residual risk: an IP address cannot be proven to belong to a GKE cluster
// offline. A kubeconfig author who supplies their own IP address and their
// own CA certificate passes the check; requiring TLS verification only
// protects against attackers on the network path. The kubeconfig cluster
// name (gke_<project>_<location>_<cluster>) is written by the same author
// and proves nothing. Deployments that accept kubeconfigs from untrusted
// parties for GKE IP endpoints should disable in-process minting or use the
// Meshery Operator.
//
// When an endpoint is refused and the plugin binary is on PATH, the plugin
// authenticates as before and the decision is logged once as a warning
// carrying ErrUntrustedClusterEndpointFallback; otherwise Apply returns
// ErrUntrustedClusterEndpoint.
//
// # Amazon EKS
//
// An EKS token is the string "k8s-aws-v1." followed by the base64 (URL
// alphabet, no padding) encoding of a presigned STS GetCallerIdentity GET URL
// for the regional STS endpoint, with the "x-k8s-aws-id: <cluster>" header
// signed into the presign and X-Amz-Expires=60. EKS accepts such a token for
// 15 minutes; the token source refreshes it one minute before that.
//
// Credentials come from the AWS SDK default chain (config.LoadDefaultConfig):
// environment variables, the shared config and credentials files and their
// profiles, web identity (IRSA, via AWS_ROLE_ARN and
// AWS_WEB_IDENTITY_TOKEN_FILE), and the ECS and EC2 instance metadata
// endpoints. When the stanza names a role (--role-arn or -r), the chain's
// credentials assume that role through STS. From the exec stanza's own env
// list, only AWS_PROFILE, AWS_REGION and AWS_DEFAULT_REGION are honored. A
// stanza whose env selects its own identity (AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN, AWS_ROLE_ARN,
// AWS_ROLE_SESSION_NAME, AWS_WEB_IDENTITY_TOKEN_FILE, AWS_CONFIG_FILE,
// AWS_SHARED_CREDENTIALS_FILE, AWS_CONTAINER_CREDENTIALS_FULL_URI,
// AWS_CONTAINER_CREDENTIALS_RELATIVE_URI, AWS_EC2_METADATA_SERVICE_ENDPOINT,
// AWS_DEFAULT_PROFILE, or HOME or USERPROFILE, which move ~/.aws) is not
// intercepted at all, so its keys are never silently replaced by the
// process identity. The region is taken from --region, then the stanza's
// AWS_REGION and AWS_DEFAULT_REGION, then the endpoint host name.
//
// # Google Kubernetes Engine
//
// A GKE token is a Google OAuth2 access token with the
// https://www.googleapis.com/auth/cloud-platform and
// https://www.googleapis.com/auth/userinfo.email scopes, the scopes
// gke-gcloud-auth-plugin requests, obtained from Application Default
// Credentials: a service-account key named by GOOGLE_APPLICATION_CREDENTIALS,
// the gcloud ADC file written by "gcloud auth application-default login",
// or the GCE/GKE metadata server (Workload Identity). On a Compute Engine
// host, both scopes must be among the instance's access scopes.
//
// Only service_account and authorized_user credential files are accepted,
// and a file may name no token_uri other than
// https://oauth2.googleapis.com/token. external_account (workload identity
// federation), impersonated_service_account and other types name further
// endpoints that would receive the process's credentials, so they are
// rejected with ErrGCPCredentialsUnavailable and the plugin is used when it
// is installed. Each refresh reloads Application Default Credentials, so a
// rewritten ADC file takes effect at the next refresh.
//
// NewGKETokenSourceFromJSON accepts service-account key JSON directly,
// under the same token_uri rule, so that a caller such as Meshery Server can
// supply credentials stored with a cloud connection. Such a source must be
// supplied through an Authenticator (by returning it from the factory passed
// to WithTokenSourceFactory) so endpoint binding still applies; it performs
// no endpoint checks of its own.
//
// # Concurrency and deadlines
//
// No lock is held across network I/O. Concurrent callers that need the same
// credential setup or the same token mint share one attempt and its result,
// success or failure, so a stalled endpoint costs one timeout, not one per
// waiting request. Every setup is bounded (30 seconds, with a 15-second
// credential check inside it) and every mint is bounded (30 seconds); the
// deadline cancels the underlying request, and a Google token exchange also
// runs on an HTTP client whose timeout ends with the deadline, because the
// service-account JWT grant is sent without a context. When a refresh fails
// while the cached token has not yet expired, the cached token keeps
// serving and the next request retries the refresh.
//
// # Missing credentials and fallback
//
// Credentials are checked when a token source is set up: AWS credentials are
// retrieved once, and Google credentials must produce one access token. If
// that fails and the plugin binary is on PATH, the Authenticator leaves the
// exec stanza in place so client-go runs the plugin as before (the common
// case on a workstation with the cloud CLI installed), and logs the setup
// failure as a warning carrying ErrInProcessCloudAuthFallback. If the binary
// is not on PATH either, Apply returns ErrInProcessCloudAuthUnavailable,
// which explains that in-process minting needs cloud credentials in the
// Meshery process and that a kubeconfig alone is not enough. Because
// DetectKubeConfig and New apply authentication while loading the
// kubeconfig, they return that error at load time rather than on the first
// request. A cached fallback decision is reused only while the stanza's own
// command is on PATH, and is reconsidered after five minutes so a transient
// setup failure does not stick.
//
// One token source is shared per cluster identity. It is set up again
// whenever it fails to produce a token or the API server answers 401, so
// credentials rewritten on disk (for example session keys refreshed in
// ~/.aws/credentials) take effect without restarting the process.
//
// Setting MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true (any value accepted by
// strconv.ParseBool) disables in-process minting entirely and leaves every
// exec stanza untouched.
//
// # Unsupported in this version
//
// AWS IAM Identity Center (SSO) sessions that need an interactive
// "aws sso login" to refresh, and assume-role configurations that require an
// MFA token code, are not supported: there is no terminal to prompt on. GKE
// stanzas that select the gcloud CLI account, impersonation, or federated
// credentials stay on the plugin. When the Meshery server process cannot
// hold cloud credentials at all, deploy the Meshery Operator into the
// cluster instead; it authenticates with the in-cluster service account and
// needs no cloud credentials or exec plugin.
package cloudauth
