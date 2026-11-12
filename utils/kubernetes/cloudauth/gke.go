package cloudauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	meshkiterrors "github.com/meshery/meshkit/errors"
)

// Use a leeway strictly smaller than oauth2.ReuseTokenSource's internal 10s
// expiry delta so an expired check in RefreshingTokenSource always triggers a
// real upstream refresh instead of hitting oauth2's inner cache repeatedly.
const gkeRefreshLeeway = 5 * time.Second

var defaultGKEScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

// GCPCredentials supplies explicit Google Cloud credentials for in-process GKE
// authentication (e.g. service-account JSON stored in a Meshery Credential).
type GCPCredentials struct {
	ServiceAccountJSON []byte
	Scopes             []string
}

// String redacts sensitive Google Cloud credential bytes.
func (c GCPCredentials) String() string {
	return fmt.Sprintf("cloudauth.GCPCredentials{HasServiceAccountJSON:%t, Scopes:%d}",
		len(c.ServiceAccountJSON) > 0, len(c.Scopes))
}

// GoString redacts sensitive Google Cloud credential bytes.
func (c GCPCredentials) GoString() string {
	return c.String()
}

type gkeConfig struct {
	findDefaultCredentials func(ctx context.Context, scopes ...string) (*google.Credentials, error)
	credentialsFromJSON    func(ctx context.Context, jsonData []byte, scopes ...string) (*google.Credentials, error)
	readCredentialsFile    func(path string) ([]byte, error)
	now                    func() time.Time
	expiryDelta            time.Duration
	mintTimeout            time.Duration
}

type gkeMintResult struct {
	creds *google.Credentials
	tok   *oauth2.Token
	err   error
}

func defaultGKEConfig() gkeConfig {
	return gkeConfig{
		findDefaultCredentials: google.FindDefaultCredentials,
		credentialsFromJSON: func(ctx context.Context, jsonData []byte, scopes ...string) (*google.Credentials, error) {
			return google.CredentialsFromJSONWithType(ctx, jsonData, google.ServiceAccount, scopes...)
		},
		readCredentialsFile: os.ReadFile,
		now:                 time.Now,
		expiryDelta:         gkeRefreshLeeway,
		mintTimeout:         defaultMintTimeout,
	}
}

func newGKETokenSource(ctx context.Context, match MatchedConfig, explicit *GCPCredentials, cfg gkeConfig) (*RefreshingTokenSource, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ErrGKETokenMint(ctx.Err())
	}

	scopes := defaultGKEScopes
	if explicit != nil && len(explicit.Scopes) > 0 {
		scopes = explicit.Scopes
	}

	mintTimeout := cfg.mintTimeout
	if mintTimeout <= 0 {
		mintTimeout = defaultMintTimeout
	}

	// golang.org/x/oauth2/google retains the resolution context for the lifetime
	// of the returned TokenSource; bind a bounded HTTP client rather than a
	// short-lived setup context so HTTP token exchanges time out cleanly without
	// canceling subsequent refreshes.
	resolve := func() (*google.Credentials, error) {
		httpClient := &http.Client{Timeout: mintTimeout}
		resolveCtx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
		gCreds, err := resolveGCPCredentials(resolveCtx, match, explicit, scopes, cfg)
		if err != nil {
			return nil, ErrGCPCredentials(err)
		}
		if gCreds == nil || gCreds.TokenSource == nil {
			return nil, ErrGCPCredentials(errors.New("resolved Google credentials have nil token source"))
		}
		return gCreds, nil
	}

	// Validate credential discovery/JSON synchronously at client construction
	// without triggering a blocking OAuth2 token exchange over the network.
	currentCreds, err := resolve()
	if err != nil {
		return nil, err
	}

	ts := NewRefreshingTokenSource(func(callCtx context.Context) (*oauth2.Token, error) {
		if callCtx == nil {
			callCtx = context.Background()
		}
		if err := callCtx.Err(); err != nil {
			return nil, ErrGKETokenMint(err)
		}

		credsSnapshot := currentCreds
		ch := make(chan gkeMintResult, 1)
		go func(creds *google.Credentials) {
			if creds == nil {
				var resolveErr error
				creds, resolveErr = resolve()
				if resolveErr != nil {
					ch <- gkeMintResult{err: resolveErr}
					return
				}
			}
			tok, tokErr := creds.TokenSource.Token()
			ch <- gkeMintResult{creds: creds, tok: tok, err: tokErr}
		}(credsSnapshot)

		select {
		case <-callCtx.Done():
			return nil, ErrGKETokenMint(callCtx.Err())
		case res := <-ch:
			if res.err != nil {
				var errV2 *meshkiterrors.ErrorV2
				if errors.As(res.err, &errV2) {
					return nil, res.err
				}
				return nil, ErrGKETokenMint(res.err)
			}
			if res.creds != nil {
				currentCreds = res.creds
			}
			if res.tok == nil || res.tok.AccessToken == "" {
				return nil, ErrGKETokenMint(errEmptyToken)
			}
			return res.tok, nil
		}
	}, cfg.expiryDelta)
	ts.onReset = func() {
		currentCreds = nil
	}
	if cfg.now != nil {
		ts.now = cfg.now
	}
	if cfg.mintTimeout > 0 {
		ts.mintTimeout = cfg.mintTimeout
	}
	return ts, nil
}

func resolveGCPCredentials(
	ctx context.Context,
	match MatchedConfig,
	explicit *GCPCredentials,
	scopes []string,
	cfg gkeConfig,
) (*google.Credentials, error) {
	if explicit != nil && len(explicit.ServiceAccountJSON) > 0 {
		fn := cfg.credentialsFromJSON
		if fn == nil {
			fn = func(c context.Context, data []byte, sc ...string) (*google.Credentials, error) {
				return google.CredentialsFromJSONWithType(c, data, google.ServiceAccount, sc...)
			}
		}
		return fn(ctx, explicit.ServiceAccountJSON, scopes...)
	}

	// Support GOOGLE_APPLICATION_CREDENTIALS passed explicitly via kubeconfig exec.env.
	if credsPath := strings.TrimSpace(match.Env["GOOGLE_APPLICATION_CREDENTIALS"]); credsPath != "" {
		readFile := cfg.readCredentialsFile
		if readFile == nil {
			readFile = os.ReadFile
		}
		data, err := readFile(credsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read GOOGLE_APPLICATION_CREDENTIALS from exec.env: %w", err)
		}
		return credentialsFromEnvJSON(ctx, data, scopes...)
	}

	findADC := cfg.findDefaultCredentials
	if findADC == nil {
		findADC = google.FindDefaultCredentials
	}
	return findADC(ctx, scopes...)
}

func credentialsFromEnvJSON(ctx context.Context, data []byte, scopes ...string) (*google.Credentials, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("invalid Google credentials JSON: %w", err)
	}
	switch google.CredentialsType(header.Type) {
	case google.ServiceAccount:
		return google.CredentialsFromJSONWithType(ctx, data, google.ServiceAccount, scopes...)
	case google.AuthorizedUser:
		return google.CredentialsFromJSONWithType(ctx, data, google.AuthorizedUser, scopes...)
	default:
		return nil, fmt.Errorf("unsupported Google credential type %q in exec.env", header.Type)
	}
}
