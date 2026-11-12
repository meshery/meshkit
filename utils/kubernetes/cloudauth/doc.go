// Package cloudauth implements in-process Kubernetes bearer token minting and
// refresh for Amazon EKS and Google GKE clusters.
//
// Supported kubeconfig credential plugins:
//   - Amazon EKS: `aws eks get-token` and `aws-iam-authenticator token`
//     (minting a presigned STS GetCallerIdentity token prefixed with
//     `k8s-aws-v1.` and signed with the `x-k8s-aws-id` header).
//   - Google GKE: `gke-gcloud-auth-plugin`, `gcloud` credential-helper
//     invocations, and legacy `auth-provider: gcp` stanzas (acquiring OAuth2
//     access tokens from Google Application Default Credentials or explicit
//     service-account JSON with `cloud-platform` and `userinfo.email` scopes).
//
// Credential boundary: static AWS keys in kubeconfig `exec.env`
// (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`) are
// deliberately ignored and never promoted into in-process credentials; use
// ambient host/process AWS credentials or explicit `WithCredentials`. Token
// minting and refresh in `RefreshingTokenSource.Token()` are bounded by a
// per-mint timeout (`defaultMintTimeout`, 30s).
//
// Unknown exec credential plugins and static bearer-token kubeconfigs are left
// untouched so client-go's exec loader (preserved via meshery/meshkit#986) and
// credential precedence continue to work as before. If in-process credentials
// are unavailable (and no explicit connection credentials were provided) while
// the exec plugin binary exists on PATH, ResolveAuthInfo also falls back to
// client-go's exec loader. Setting MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true
// disables in-process minting.
//
// # Limitations (v1)
//
// Interactive AWS SSO, MFA token prompts, and interactive `gcloud auth login`
// flows are not supported in v1. Clusters whose credentials rely on those
// flows should use non-interactive credentials (IAM keys, AssumeRole, IRSA,
// GCP service account JSON, Workload Identity) or deploy Meshery Operator in
// cluster.
package cloudauth
