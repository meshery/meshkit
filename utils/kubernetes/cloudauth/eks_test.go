package cloudauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/errors"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func notFoundLookPath(file string) (string, error) {
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

func isolateAWSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"AWS_DEFAULT_PROFILE",
		"AWS_CONFIG_FILE",
		"AWS_SHARED_CREDENTIALS_FILE",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_ROLE_ARN",
		"AWS_ROLE_SESSION_NAME",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_STS_REGIONAL_ENDPOINTS",
		DisableEnvVar,
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
}

func eksExecAuthInfo(cluster, region string) *clientcmdapi.AuthInfo {
	args := []string{"eks", "get-token", "--cluster-name", cluster}
	if region != "" {
		args = append(args, "--region", region)
	}
	return &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command: "aws",
			Args:    args,
		},
	}
}

func decodeEKSToken(t *testing.T, bearer string) *url.URL {
	t.Helper()
	if !strings.HasPrefix(bearer, eksTokenPrefix) {
		t.Fatalf("token %q missing %q prefix", bearer, eksTokenPrefix)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(bearer, eksTokenPrefix))
	if err != nil {
		t.Fatalf("RawURLEncoding.DecodeString error: %v", err)
	}
	parsed, err := url.Parse(string(raw))
	if err != nil {
		t.Fatalf("url.Parse(%q) error: %v", string(raw), err)
	}
	return parsed
}

func TestEKSTokenPresignFormatAndProactiveRefresh(t *testing.T) {
	isolateAWSEnv(t)

	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	ts, matched, ok, err := ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("demo-eks", "eu-central-1"),
		WithCredentials(Credentials{
			AWS: &AWSCredentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		}),
		withDisableIMDS(),
		withClock(func() time.Time { return now }),
	)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}
	if matched.Provider != ProviderEKS {
		t.Fatalf("Provider = %q, want eks", matched.Provider)
	}

	tok1, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	parsed := decodeEKSToken(t, tok1.AccessToken)

	if parsed.Scheme != "https" || parsed.Host != "sts.eu-central-1.amazonaws.com" {
		t.Fatalf("unexpected presigned host/scheme: %s://%s", parsed.Scheme, parsed.Host)
	}
	q := parsed.Query()
	if q.Get("Action") != "GetCallerIdentity" {
		t.Fatalf("Action = %q, want GetCallerIdentity", q.Get("Action"))
	}
	if q.Get("X-Amz-Expires") != "60" {
		t.Fatalf("X-Amz-Expires = %q, want 60", q.Get("X-Amz-Expires"))
	}
	signedHeaders := strings.ToLower(q.Get("X-Amz-SignedHeaders"))
	if !strings.Contains(signedHeaders, "x-k8s-aws-id") || !strings.Contains(signedHeaders, "host") {
		t.Fatalf("X-Amz-SignedHeaders = %q, want host and x-k8s-aws-id", signedHeaders)
	}
	if q.Has("x-k8s-aws-id") {
		t.Fatal("x-k8s-aws-id leaked into query parameters; must be a signed header")
	}
	if q.Get("X-Amz-Signature") == "" {
		t.Fatal("presigned URL missing X-Amz-Signature")
	}
	if cred := q.Get("X-Amz-Credential"); !strings.Contains(cred, "/eu-central-1/sts/aws4_request") {
		t.Fatalf("X-Amz-Credential = %q, want eu-central-1/sts/aws4_request scope", cred)
	}
	if tok1.Expiry != now.Add(15*time.Minute) {
		t.Fatalf("Expiry = %v, want %v", tok1.Expiry, now.Add(15*time.Minute))
	}

	// Within validity window -> exact cached token returned.
	now = now.Add(10 * time.Minute)
	tokCached, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() cached error: %v", err)
	}
	if tokCached.AccessToken != tok1.AccessToken {
		t.Fatal("expected identical cached token before refresh window")
	}

	// Cross 2m proactive refresh threshold -> new token minted with updated expiry.
	now = now.Add(3 * time.Minute) // now is base + 13m
	tok2, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() refreshed error: %v", err)
	}
	if !strings.HasPrefix(tok2.AccessToken, eksTokenPrefix) {
		t.Fatalf("refreshed token missing prefix: %q", tok2.AccessToken)
	}
	if tok2.Expiry != now.Add(15*time.Minute) {
		t.Fatalf("refreshed Expiry = %v, want %v", tok2.Expiry, now.Add(15*time.Minute))
	}
}

func TestEKSAssumeRoleViaMockSTS(t *testing.T) {
	isolateAWSEnv(t)

	var assumeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("Action") != "AssumeRole" {
			http.Error(w, "unexpected Action: "+r.Form.Get("Action"), http.StatusBadRequest)
			return
		}
		if got := r.Form.Get("RoleArn"); got != "arn:aws:iam::123456789012:role/eks-access" {
			http.Error(w, "unexpected RoleArn: "+got, http.StatusBadRequest)
			return
		}
		if got := r.Form.Get("ExternalId"); got != "meshery-ext" {
			http.Error(w, "unexpected ExternalId: "+got, http.StatusBadRequest)
			return
		}
		assumeCalls.Add(1)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprint(w, `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>ASIATEMPACCESSKEY123</AccessKeyId>
      <SecretAccessKey>temp-secret-access-key</SecretAccessKey>
      <SessionToken>mock-assumed-session-token</SessionToken>
      <Expiration>2099-01-01T00:00:00Z</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <AssumedRoleId>AROADEMO:meshery</AssumedRoleId>
      <Arn>arn:aws:sts::123456789012:assumed-role/eks-access/meshery</Arn>
    </AssumedRoleUser>
  </AssumeRoleResult>
</AssumeRoleResponse>`)
	}))
	defer srv.Close()

	authInfo := &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command: "aws-iam-authenticator",
			Args: []string{
				"token",
				"-i", "assumed-cluster",
				"-r", "arn:aws:iam::123456789012:role/eks-access",
				"--external-id", "meshery-ext",
				"--session-name", "meshery",
			},
		},
	}

	ts, _, ok, err := ResolveAuthInfo(
		context.Background(),
		authInfo,
		WithCredentials(Credentials{
			AWS: &AWSCredentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				Region:          "us-east-1",
				STSEndpoint:     srv.URL,
			},
		}),
		withDisableIMDS(),
	)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}
	if assumeCalls.Load() != 1 {
		t.Fatalf("AssumeRole calls = %d, want 1", assumeCalls.Load())
	}

	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	parsed := decodeEKSToken(t, tok.AccessToken)
	if got := parsed.Query().Get("X-Amz-Security-Token"); got != "mock-assumed-session-token" {
		t.Fatalf("X-Amz-Security-Token = %q, want mock-assumed-session-token", got)
	}
}

func TestEKSWebIdentityIRSAFromExecEnv(t *testing.T) {
	isolateAWSEnv(t)

	var webIDCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("Action") != "AssumeRoleWithWebIdentity" {
			http.Error(w, "unexpected Action: "+r.Form.Get("Action"), http.StatusBadRequest)
			return
		}
		if r.Form.Get("WebIdentityToken") != "oidc-jwt-token" {
			http.Error(w, "unexpected WebIdentityToken", http.StatusBadRequest)
			return
		}
		webIDCalls.Add(1)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprint(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleWithWebIdentityResult>
    <Credentials>
      <AccessKeyId>ASIAIRSAKEY123456</AccessKeyId>
      <SecretAccessKey>irsa-secret-access-key</SecretAccessKey>
      <SessionToken>mock-irsa-session-token</SessionToken>
      <Expiration>2099-01-01T00:00:00Z</Expiration>
    </Credentials>
  </AssumeRoleWithWebIdentityResult>
</AssumeRoleWithWebIdentityResponse>`)
	}))
	defer srv.Close()

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("oidc-jwt-token"), 0o600); err != nil {
		t.Fatalf("WriteFile token: %v", err)
	}

	authInfo := &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command: "aws",
			Args:    []string{"eks", "get-token", "--cluster-name", "irsa-cluster", "--endpoint-url", srv.URL},
			Env: []clientcmdapi.ExecEnvVar{
				{Name: "AWS_WEB_IDENTITY_TOKEN_FILE", Value: tokenPath},
				{Name: "AWS_ROLE_ARN", Value: "arn:aws:iam::123456789012:role/irsa-role"},
			},
		},
	}

	ts, _, ok, err := ResolveAuthInfo(context.Background(), authInfo, withDisableIMDS())
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}
	if webIDCalls.Load() != 1 {
		t.Fatalf("AssumeRoleWithWebIdentity calls = %d, want 1", webIDCalls.Load())
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	parsed := decodeEKSToken(t, tok.AccessToken)
	if got := parsed.Query().Get("X-Amz-Security-Token"); got != "mock-irsa-session-token" {
		t.Fatalf("X-Amz-Security-Token = %q, want mock-irsa-session-token", got)
	}
}

func TestEKSMissingCredentialsAndErrorClassification(t *testing.T) {
	isolateAWSEnv(t)

	// Missing credentials with IMDS disabled and no binary on PATH fails cleanly.
	_, _, ok, err := ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("no-creds", "us-east-1"),
		withDisableIMDS(),
		withLookPath(notFoundLookPath),
	)
	if !ok || err == nil {
		t.Fatalf("expected error, got ok=%v err=%v", ok, err)
	}
	if got := errors.GetCode(err); got != ErrAWSCredentialsCode {
		t.Fatalf("code = %q, want %q (err=%v)", got, ErrAWSCredentialsCode, err)
	}

	// Valid credentials without any AWS region fails with ErrEKSRegionUnresolved.
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("no-region", ""),
		WithCredentials(Credentials{
			AWS: &AWSCredentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		}),
		withDisableIMDS(),
		withLookPath(notFoundLookPath),
	)
	if !ok || err == nil {
		t.Fatalf("expected missing region error, got ok=%v err=%v", ok, err)
	}
	if got := errors.GetCode(err); got != ErrEKSRegionUnresolvedCode {
		t.Fatalf("missing region code = %q, want %q (err=%v)", got, ErrEKSRegionUnresolvedCode, err)
	}

	// Incomplete explicit static keys fail cleanly.
	_, _, _, err = ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("partial-creds", "us-east-1"),
		WithCredentials(Credentials{AWS: &AWSCredentials{AccessKeyID: "AKIAONLY"}}),
		withDisableIMDS(),
	)
	if got := errors.GetCode(err); got != ErrAWSCredentialsCode {
		t.Fatalf("incomplete keys code = %q, want %q", got, ErrAWSCredentialsCode)
	}

	// SSO/MFA errors classify as unsupported flow.
	ssoErr := classifyAWSError(fmt.Errorf("failed to refresh cached credentials, the SSO session has expired"), true)
	if got := errors.GetCode(ssoErr); got != ErrUnsupportedCloudAuthFlowCode {
		t.Fatalf("sso error code = %q, want %q", got, ErrUnsupportedCloudAuthFlowCode)
	}
}

func TestEKSFallbackOnPathAndDisableEnvVar(t *testing.T) {
	isolateAWSEnv(t)

	foundOnPath := func(cmd string) (string, error) {
		return "/usr/local/bin/" + cmd, nil
	}

	// Missing in-process creds with aws CLI on PATH falls back cleanly to #986.
	_, _, ok, err := ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("cli-fallback", "us-east-1"),
		withDisableIMDS(),
		withLookPath(foundOnPath),
	)
	if ok || err != nil {
		t.Fatalf("expected PATH fallback (matched=false, err=nil), got ok=%v err=%v", ok, err)
	}

	// --sso flag with aws CLI on PATH also falls back cleanly to #986.
	ssoAuth := eksExecAuthInfo("sso-fallback", "us-east-1")
	ssoAuth.Exec.Args = append(ssoAuth.Exec.Args, "--sso")
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		ssoAuth,
		withDisableIMDS(),
		withLookPath(foundOnPath),
	)
	if ok || err != nil {
		t.Fatalf("expected SSO PATH fallback, got ok=%v err=%v", ok, err)
	}

	// Explicit WithCredentials must NOT silently fall back to PATH binary on failure.
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("explicit-err", "us-east-1"),
		WithCredentials(Credentials{AWS: &AWSCredentials{AccessKeyID: "AKIAONLY"}}),
		withDisableIMDS(),
		withLookPath(foundOnPath),
	)
	if !ok || errors.GetCode(err) != ErrAWSCredentialsCode {
		t.Fatalf("expected ErrAWSCredentials with explicit creds, got ok=%v err=%v", ok, err)
	}

	// MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH disables in-process resolution completely.
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		eksExecAuthInfo("disabled", "us-east-1"),
		WithCredentials(Credentials{
			AWS: &AWSCredentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		}),
		withGetenv(func(k string) string {
			if k == DisableEnvVar {
				return "true"
			}
			return ""
		}),
	)
	if ok || err != nil {
		t.Fatalf("expected disabled in-process auth (matched=false, err=nil), got ok=%v err=%v", ok, err)
	}
}
