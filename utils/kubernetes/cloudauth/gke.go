package cloudauth

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// gkeScope is the OAuth2 scope GKE accepts for Kubernetes API access, the
// same one gke-gcloud-auth-plugin requests.
const gkeScope = "https://www.googleapis.com/auth/cloud-platform"

// NewGKETokenSource returns a token source backed by Google Application
// Default Credentials: GOOGLE_APPLICATION_CREDENTIALS, the gcloud ADC file,
// or the GCE/GKE metadata server (Workload Identity). Tokens are cached and
// refreshed automatically.
//
// ctx is retained by the credentials for later token refreshes, so it must
// not be cancelled while the token source is in use.
//
// It returns ErrGCPCredentialsUnavailable when no Application Default
// Credentials can be found.
func NewGKETokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	creds, err := google.FindDefaultCredentials(ctx, gkeScope)
	if err != nil {
		return nil, ErrGCPCredentialsUnavailable(err)
	}
	return oauth2.ReuseTokenSource(nil, creds.TokenSource), nil
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
