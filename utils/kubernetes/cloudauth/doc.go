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
//     --region).
//   - Google Kubernetes Engine: "gke-gcloud-auth-plugin", or "gcloud" used as
//     a credential plugin (for example "gcloud config config-helper").
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
// list, AWS_PROFILE, AWS_REGION, AWS_DEFAULT_REGION, AWS_CONFIG_FILE,
// AWS_SHARED_CREDENTIALS_FILE and the static key triple (AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN) are honored. The region is taken
// from --region, then the stanza's AWS_REGION and AWS_DEFAULT_REGION, then
// the process environment, then the shared config profile.
//
// # Google Kubernetes Engine
//
// A GKE token is a Google OAuth2 access token with the
// https://www.googleapis.com/auth/cloud-platform scope, obtained from
// Application Default Credentials (google.FindDefaultCredentials): a
// service-account key named by GOOGLE_APPLICATION_CREDENTIALS, the gcloud ADC
// file written by "gcloud auth application-default login", or the GCE/GKE
// metadata server (Workload Identity). This matches gke-gcloud-auth-plugin
// run with --use_application_default_credentials; the identity of the
// gcloud CLI's active account is not consulted. NewGKETokenSourceFromJSON
// accepts service-account key JSON directly so that a caller, such as Meshery
// Server, can supply credentials stored with a cloud connection.
//
// # Missing credentials and fallback
//
// Credentials are checked when a token source is set up: AWS credentials are
// retrieved once with a timeout, and Google ADC must produce one access token
// within a timeout. If that
// fails and the plugin binary is on PATH, the Authenticator leaves the exec
// stanza in place so client-go runs the plugin as before (the common case on
// a workstation with the cloud CLI installed). If the binary is not on PATH
// either, Apply returns ErrInProcessCloudAuthUnavailable, which explains that
// in-process minting needs cloud credentials in the Meshery process and that
// a kubeconfig alone is not enough.
//
// Setting MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true (any value accepted by
// strconv.ParseBool) disables in-process minting entirely and leaves every
// exec stanza untouched.
//
// # Unsupported in this version
//
// AWS IAM Identity Center (SSO) sessions that need an interactive
// "aws sso login" to refresh, and assume-role configurations that require an
// MFA token code, are not supported: there is no terminal to prompt on. When
// the Meshery server process cannot hold cloud credentials at all, deploy the
// Meshery Operator into the cluster instead; it authenticates with the
// in-cluster service account and needs no cloud credentials or exec plugin.
package cloudauth
