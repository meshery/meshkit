package cloudauth

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/meshery/meshkit/errors"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DisableEnvVar names the environment variable that, when set to a truthy value
// (parsed via strconv.ParseBool), disables in-process cloud token minting and
// leaves every kubeconfig stanza untouched on client-go's loader path.
const DisableEnvVar = "MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH"

// Credentials holds optional per-connection cloud credentials supplied in memory
// (never persisted to disk or written into process environment variables).
type Credentials struct {
	AWS *AWSCredentials
	GCP *GCPCredentials
}

// String redacts secret credential material when formatting.
func (c Credentials) String() string {
	hasAWS := c.AWS != nil
	hasGCP := c.GCP != nil
	return "cloudauth.Credentials{HasAWS:" + boolStr(hasAWS) + ", HasGCP:" + boolStr(hasGCP) + "}"
}

// GoString redacts secret credential material when formatting.
func (c Credentials) GoString() string {
	return c.String()
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// Option customizes in-process cloud authentication behavior (primarily used for
// testing or per-call overrides).
type Option func(*options)

type options struct {
	credentials Credentials
	eks         eksConfig
	gke         gkeConfig
	lookPath    func(string) (string, error)
	getenv      func(string) string
}

func defaultOptions() options {
	return options{
		eks:      defaultEKSConfig(),
		gke:      defaultGKEConfig(),
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
	}
}

// WithCredentials supplies explicit per-connection AWS/GCP credentials.
func WithCredentials(creds Credentials) Option {
	return func(o *options) {
		o.credentials = creds
	}
}

// ResolveAuthInfo inspects authInfo and, if it matches a supported EKS or GKE
// authentication stanza, builds an in-process refreshing TokenSource.
//
// When matched == false and err == nil (e.g. unknown plugin, disabled via
// DisableEnvVar, or missing in-process credentials when the exec plugin binary
// is available on PATH), callers must leave authInfo untouched on client-go's
// existing #986 kubeconfig loader path.
func ResolveAuthInfo(ctx context.Context, authInfo *clientcmdapi.AuthInfo, optFns ...Option) (*RefreshingTokenSource, MatchedConfig, bool, error) {
	opts := defaultOptions()
	for _, fn := range optFns {
		if fn != nil {
			fn(&opts)
		}
	}

	if disabled, parseErr := strconv.ParseBool(strings.TrimSpace(opts.getenv(DisableEnvVar))); parseErr == nil && disabled {
		return nil, MatchedConfig{}, false, nil
	}

	match, matched, err := MatchAuthInfo(authInfo)
	if !matched {
		return nil, match, false, nil
	}
	if err != nil {
		hasExplicit := (match.Provider == ProviderEKS && opts.credentials.AWS != nil) ||
			(match.Provider == ProviderGKE && opts.credentials.GCP != nil)
		if shouldFallbackToExec(authInfo, opts, err, hasExplicit) {
			return nil, MatchedConfig{}, false, nil
		}
		return nil, match, true, err
	}

	switch match.Provider {
	case ProviderEKS:
		ts, eksErr := newEKSTokenSource(ctx, match, opts.credentials.AWS, opts.eks)
		if eksErr != nil && shouldFallbackToExec(authInfo, opts, eksErr, opts.credentials.AWS != nil) {
			return nil, MatchedConfig{}, false, nil
		}
		return ts, match, true, eksErr
	case ProviderGKE:
		ts, gkeErr := newGKETokenSource(ctx, match, opts.credentials.GCP, opts.gke)
		if gkeErr != nil && shouldFallbackToExec(authInfo, opts, gkeErr, opts.credentials.GCP != nil) {
			return nil, MatchedConfig{}, false, nil
		}
		return ts, match, true, gkeErr
	default:
		return nil, MatchedConfig{}, false, nil
	}
}

// shouldFallbackToExec checks whether a failed in-process match or setup should
// yield to client-go's exec plugin loader (#986):
//   - Only exec stanzas (not legacy auth-provider: gcp, which client-go removed)
//   - Only when no explicit per-connection credentials were passed
//   - Never for structurally invalid stanzas missing required cluster flags
//   - Only when the exec command binary is resolvable via lookPath on the host
func shouldFallbackToExec(authInfo *clientcmdapi.AuthInfo, opts options, err error, hasExplicitCreds bool) bool {
	if hasExplicitCreds || authInfo == nil || authInfo.Exec == nil || opts.lookPath == nil {
		return false
	}
	if errors.GetCode(err) == ErrInvalidCloudExecConfigCode {
		return false
	}
	_, lookErr := opts.lookPath(authInfo.Exec.Command)
	return lookErr == nil
}
