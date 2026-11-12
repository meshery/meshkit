package cloudauth

import (
	"context"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// gkeScope is the OAuth2 scope GKE accepts for Kubernetes API access,
	// the same one gke-gcloud-auth-plugin requests.
	gkeScope = "https://www.googleapis.com/auth/cloud-platform"
	// gcpCredentialCheckTimeout bounds the token fetch done at setup, which
	// may exchange a refresh token, sign a JWT grant, or query the metadata
	// server.
	gcpCredentialCheckTimeout = 15 * time.Second
)

// NewGKETokenSource returns a token source backed by Google Application
// Default Credentials: GOOGLE_APPLICATION_CREDENTIALS, the gcloud ADC file,
// or the GCE/GKE metadata server (Workload Identity). It fetches one token
// up front, bounded by ctx and a timeout, so credentials that can be found
// but not used (for example a revoked refresh token) are reported now rather
// than on the first request. Tokens are cached and refreshed automatically.
//
// ctx bounds only that setup. The credentials keep its values but not its
// cancellation for later refreshes, so the source keeps working after ctx
// ends.
//
// It returns ErrGCPCredentialsUnavailable when no Application Default
// Credentials can be found or they cannot produce a token.
func NewGKETokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	creds, err := google.FindDefaultCredentials(context.WithoutCancel(ctx), gkeScope)
	if err != nil {
		return nil, ErrGCPCredentialsUnavailable(err)
	}
	token, err := fetchToken(ctx, creds.TokenSource)
	if err != nil {
		return nil, ErrGCPCredentialsUnavailable(err)
	}
	return oauth2.ReuseTokenSource(token, creds.TokenSource), nil
}

// fetchToken calls source.Token, giving up when ctx ends or after
// gcpCredentialCheckTimeout. oauth2.TokenSource takes no context, and the
// metadata server path ignores the one the credentials hold, so the call
// runs in a goroutine that is abandoned on timeout.
func fetchToken(ctx context.Context, source oauth2.TokenSource) (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(ctx, gcpCredentialCheckTimeout)
	defer cancel()

	type result struct {
		token *oauth2.Token
		err   error
	}
	done := make(chan result, 1)
	go func() {
		token, err := source.Token()
		done <- result{token: token, err: err}
	}()

	select {
	case r := <-done:
		return r.token, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// NewGKETokenSourceFromJSON returns a token source backed by the given
// service-account key JSON, for callers (such as Meshery Server) that hold
// credentials stored with a cloud connection rather than ADC. Only the
// service_account credential type is accepted, so a key cannot redirect
// token exchange to an arbitrary endpoint or local file. The JSON is
// validated now; no network call is made until the first Token call.
//
// ctx is retained by the credentials for later token refreshes, so it must
// not be cancelled while the token source is in use.
func NewGKETokenSourceFromJSON(ctx context.Context, serviceAccountKey []byte) (oauth2.TokenSource, error) {
	creds, err := google.CredentialsFromJSONWithType(ctx, serviceAccountKey, google.ServiceAccount, gkeScope)
	if err != nil {
		return nil, ErrGCPServiceAccountKey(err)
	}
	return oauth2.ReuseTokenSource(nil, creds.TokenSource), nil
}
