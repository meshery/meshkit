package cloudauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// gcpCredentialCheckTimeout bounds the token fetch done at setup, which
	// may exchange a refresh token, sign a JWT grant, or query the metadata
	// server.
	gcpCredentialCheckTimeout = 15 * time.Second
	// googleTokenURL is the only token endpoint a credentials file may name.
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

// gkeScopes are the OAuth2 scopes GKE requires: cloud-platform for API
// access and userinfo.email so the API server can identify the caller. They
// are the scopes gke-gcloud-auth-plugin requests.
var gkeScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

// googleTransport is the HTTP transport for Google OAuth2 token exchanges;
// nil means http.DefaultTransport. Tests replace it to answer for
// oauth2.googleapis.com without the network.
var googleTransport http.RoundTripper

// googleCredentialsLoader loads the credentials a GKE token source mints
// from. It is called once per mint, with that mint's context.
type googleCredentialsLoader func(ctx context.Context) (*google.Credentials, error)

// setupGKETokenSource returns a token source backed by Google Application
// Default Credentials: GOOGLE_APPLICATION_CREDENTIALS, the gcloud ADC file,
// or the GCE/GKE metadata server (Workload Identity). Tokens carry the
// cloud-platform and userinfo.email scopes; on the metadata server both
// must be among the host's access scopes (GKE Workload Identity grants
// them).
//
// Only service_account keys and authorized_user (gcloud ADC login) files are
// accepted, and a file may name no token endpoint other than
// https://oauth2.googleapis.com/token. external_account (workload identity
// federation), impersonated_service_account and every other credential type
// is rejected, because each names further endpoints that would receive the
// process's credentials; stanzas needing them stay on the exec plugin when
// it is installed.
//
// One token is fetched up front, bounded by ctx and a timeout and cancelled
// with them, so credentials that can be found but not used (for example a
// revoked refresh token) are reported now rather than on the first request.
// ctx bounds only that setup: every later refresh reloads the credentials
// (so a rewritten ADC file is picked up) under its own deadline.
//
// It returns ErrGCPCredentialsUnavailable when no usable Application Default
// Credentials can be found or they cannot produce a token.
func setupGKETokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	return newGKETokenSource(ctx, loadApplicationDefaultCredentials, ErrGCPCredentialsUnavailable)
}

// NewGKETokenSourceFromJSON returns a token source backed by the given
// service-account key JSON, for callers (such as Meshery Server) that hold
// credentials stored with a cloud connection rather than ADC. Only the
// service_account credential type is accepted, and its token_uri must be
// empty or https://oauth2.googleapis.com/token, so a key cannot redirect its
// signed JWT grant to another endpoint.
//
// Like setupGKETokenSource it fetches one token up front, bounded by ctx and a
// timeout, so an unusable key fails now; ctx bounds only that setup.
//
// The returned source performs no endpoint checks of its own. Supply it
// through an Authenticator, by returning it from the factory passed to
// WithTokenSourceFactory, so its tokens reach only trusted GKE endpoints;
// attaching it to a rest.Config directly would send them to whatever server
// the config names.
func NewGKETokenSourceFromJSON(ctx context.Context, serviceAccountKey []byte) (oauth2.TokenSource, error) {
	if _, err := checkGoogleCredentialsJSON(serviceAccountKey, google.ServiceAccount); err != nil {
		return nil, ErrGCPServiceAccountKey(err)
	}
	return newGKETokenSource(ctx, func(ctx context.Context) (*google.Credentials, error) {
		return google.CredentialsFromJSONWithType(ctx, serviceAccountKey, google.ServiceAccount, gkeScopes...)
	}, ErrGCPServiceAccountKey)
}

// newGKETokenSource fetches one token from load under ctx and the setup
// timeout, and returns a caching source seeded with it whose refreshes call
// load again under the mint deadline. wrap turns a failure into the
// MeshKit error for the credential kind.
func newGKETokenSource(ctx context.Context, load googleCredentialsLoader, wrap func(error) error) (*cachingTokenSource, error) {
	source := newCachingTokenSource(func(ctx context.Context, _ time.Time) (*oauth2.Token, error) {
		token, err := mintGoogleToken(ctx, load)
		if err != nil {
			return nil, wrap(err)
		}
		return token, nil
	})

	setupCtx, cancel := context.WithTimeout(ctx, gcpCredentialCheckTimeout)
	defer cancel()
	token, err := mintGoogleToken(setupCtx, load)
	if err != nil {
		return nil, wrap(err)
	}
	source.token = token
	source.mintedAt = source.clock.Now()
	return source, nil
}

// mintGoogleToken loads credentials and fetches one fresh token, both under
// ctx. golang.org/x/oauth2 sends some token requests without a context (the
// service-account JWT grant), so the credentials also get an HTTP client
// whose timeout ends with ctx's deadline: that is what actually aborts a
// stalled exchange. The metadata server client has its own timeouts.
func mintGoogleToken(ctx context.Context, load googleCredentialsLoader) (*oauth2.Token, error) {
	timeout := defaultMintTimeout
	if deadline, ok := ctx.Deadline(); ok {
		// A zero or negative Timeout would mean no timeout at all.
		timeout = max(time.Until(deadline), time.Millisecond)
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: googleTransport, Timeout: timeout})

	return runBounded(ctx, func(ctx context.Context) (*oauth2.Token, error) {
		creds, err := load(ctx)
		if err != nil {
			return nil, err
		}
		// Each load builds a new token source with no cached token, so
		// this is always a fresh exchange.
		return creds.TokenSource.Token()
	})
}

// loadApplicationDefaultCredentials finds Application Default Credentials
// the way google.FindDefaultCredentials does and rejects credential files
// this package does not accept (see setupGKETokenSource). Metadata server
// credentials carry no JSON and are always accepted.
func loadApplicationDefaultCredentials(ctx context.Context) (*google.Credentials, error) {
	creds, err := google.FindDefaultCredentials(ctx, gkeScopes...)
	if err != nil {
		return nil, err
	}
	if len(creds.JSON) == 0 {
		return creds, nil
	}
	if _, err := checkGoogleCredentialsJSON(creds.JSON, google.ServiceAccount, google.AuthorizedUser); err != nil {
		return nil, err
	}
	return creds, nil
}

// checkGoogleCredentialsJSON returns the credential type of data when it is
// one of allowed and names no token endpoint other than googleTokenURL.
func checkGoogleCredentialsJSON(data []byte, allowed ...google.CredentialsType) (google.CredentialsType, error) {
	var file struct {
		Type     string `json:"type"`
		TokenURI string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return "", fmt.Errorf("the credentials are not valid JSON: %w", err)
	}
	credType := google.CredentialsType(file.Type)
	supported := false
	for _, t := range allowed {
		supported = supported || credType == t
	}
	if !supported {
		return "", fmt.Errorf("credential type %q is not supported for in-process GKE token minting (supported: %v)", file.Type, allowed)
	}
	if file.TokenURI != "" && file.TokenURI != googleTokenURL {
		return "", fmt.Errorf("token_uri %q is not the Google OAuth2 token endpoint %s", file.TokenURI, googleTokenURL)
	}
	return credType, nil
}
