package kubernetes

import (
	"errors"
	"os"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Test design note:
//
// We intentionally assert behavior-level invariants for setupKubeConfig()
// instead of doing a full DeepEqual on *genericclioptions.ConfigFlags for the
// following reasons
//
// 1. the function produces runtime-generated artifacts (temp file paths)
// for CA/Cert/Key material. Those paths are nondeterministic by design, so exact
// object equality would make tests brittle and OS/environment-dependent.
//
// 2. the function returns a cleanup closure. Function values are not
// meaningfully comparable for equality in Go, so "exact output object" checking
// is not a stable assertion model for this API shape.
//
// 3. the real contract of this branch is not "exact struct bytes match";
// it is: select non-exec mode, force KubeConfig=/dev/null, propagate auth/TLS
// fields correctly, create material files only when required, and provide a
// cleanup hook that removes created files.
//
// This test therefore checks deterministic outcomes that represent that contract:
// field propagation, branch selection, file-presence/file-absence expectations,
// and cleanup side effects.
//
// If we later introduce dependency injection for temp-file creation, we can add
// stricter deterministic assertions (including exact file names/paths) without
// changing the behavior contract validated here.
func Test_setupKubeConfig(t *testing.T) {
	type (
		branchKind string
		pathKind   string
		// only for deterministic error injection in tests
		failPoint string
	)

	const (
		branchNonExec branchKind = "non-exec"
		branchExec    branchKind = "exec"
		pathSkip      pathKind   = "skip"
		pathNil       pathKind   = "nil"
		pathDevNull   pathKind   = "devnull"
		pathTempFile  pathKind   = "tempFile"

		// only for deterministic error injection in tests
		failNone                failPoint = ""
		failCADataTempFile      failPoint = "ca-tempfile"
		failCertDataTempFile    failPoint = "cert-tempfile"
		failKeyDataTempFile     failPoint = "key-tempfile"
		failExecConfigSerialize failPoint = "exec-config-serialize"
		failExecConfigTempFile  failPoint = "exec-config-tempfile"
	)

	type opt[T any] struct {
		enabled bool
		value   T
	}

	type pathWant struct {
		enabled bool
		kind    pathKind
	}

	type wantErr struct {
		want     bool
		contains string // optional substring check
	}

	type want struct {
		branch branchKind
		err    wantErr

		apiServer   opt[string]
		bearerToken opt[string]
		insecure    opt[bool]
		username    opt[string]
		password    opt[string]

		kubeConfig pathWant
		caFile     pathWant
		certFile   pathWant
		keyFile    pathWant
	}

	setupFailPoint := func(t *testing.T, fp failPoint) func() {
		t.Helper()

		originalWriteKubeConfig := writeKubeConfig
		originalWriteTempData := writeTempData
		restore := func() {
			writeKubeConfig = originalWriteKubeConfig
			writeTempData = originalWriteTempData
		}

		switch fp {
		case failNone:
			return restore
		case failExecConfigSerialize:
			writeKubeConfig = func(_ clientcmdapi.Config) ([]byte, error) {
				return nil, errors.New("injected kubeconfig serialization error")
			}
		case failCADataTempFile, failCertDataTempFile, failKeyDataTempFile, failExecConfigTempFile:
			targetCall := 1
			switch fp {
			case failCertDataTempFile:
				targetCall = 2
			case failKeyDataTempFile:
				targetCall = 3
			}
			callCount := 0
			writeTempData = func(data []byte) (string, error) {
				callCount++
				if callCount == targetCall {
					return "", errors.New("injected temp file error")
				}
				return originalWriteTempData(data)
			}
		default:
			t.Fatalf("unknown failpoint %q", fp)
		}

		return restore
	}

	tests := []struct {
		name   string
		client *Client
		failAt failPoint // requires DI hooks in test harness; keep failNone for normal cases
		want   want
	}{
		{
			name:   "non-exec branch selected when exec provider is nil and bearer token is empty",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.empty-token.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:      branchNonExec,
				err:         wantErr{want: false},
				apiServer:   opt[string]{enabled: true, value: "https://api.empty-token.local:6443"},
				bearerToken: opt[string]{enabled: true, value: ""},
				insecure:    opt[bool]{enabled: true, value: true},
				kubeConfig:  pathWant{enabled: true, kind: pathDevNull},
				caFile:      pathWant{enabled: true, kind: pathNil},
				certFile:    pathWant{enabled: true, kind: pathNil},
				keyFile:     pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch propagates api server and bearer token",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:        "https://127.0.0.1:6443",
					BearerToken: "token-123",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:      branchNonExec,
				err:         wantErr{want: false},
				apiServer:   opt[string]{enabled: true, value: "https://127.0.0.1:6443"},
				bearerToken: opt[string]{enabled: true, value: "token-123"},
				insecure:    opt[bool]{enabled: true, value: true},
				kubeConfig:  pathWant{enabled: true, kind: pathDevNull},
				caFile:      pathWant{enabled: true, kind: pathNil},
				certFile:    pathWant{enabled: true, kind: pathNil},
				keyFile:     pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch is still selected when exec provider exists but bearer token is non-empty",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:        "https://api.nonexec.local:6443",
					BearerToken: "static-token",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "aws",
						Args:       []string{"eks", "get-token"},
						APIVersion: "client.authentication.k8s.io/v1",
					},
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:      branchNonExec,
				err:         wantErr{want: false},
				apiServer:   opt[string]{enabled: true, value: "https://api.nonexec.local:6443"},
				bearerToken: opt[string]{enabled: true, value: "static-token"},
				insecure:    opt[bool]{enabled: true, value: true},
				kubeConfig:  pathWant{enabled: true, kind: pathDevNull},
				caFile:      pathWant{enabled: true, kind: pathNil},
				certFile:    pathWant{enabled: true, kind: pathNil},
				keyFile:     pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch treats whitespace bearer token as non-empty and avoids exec branch",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:        "https://api.whitespace-token.local:6443",
					BearerToken: " ",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "aws",
						Args:       []string{"eks", "get-token"},
						APIVersion: "client.authentication.k8s.io/v1",
					},
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:      branchNonExec,
				err:         wantErr{want: false},
				apiServer:   opt[string]{enabled: true, value: "https://api.whitespace-token.local:6443"},
				bearerToken: opt[string]{enabled: true, value: " "},
				insecure:    opt[bool]{enabled: true, value: true},
				kubeConfig:  pathWant{enabled: true, kind: pathDevNull},
				caFile:      pathWant{enabled: true, kind: pathNil},
				certFile:    pathWant{enabled: true, kind: pathNil},
				keyFile:     pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch sets username only when password is empty",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:     "https://api.user-only.local:6443",
					Username: "alice",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.user-only.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				username:   opt[string]{enabled: true, value: "alice"},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch sets password only when username is empty",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:     "https://api.pass-only.local:6443",
					Password: "super-secret",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.pass-only.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				password:   opt[string]{enabled: true, value: "super-secret"},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch sets both username and password when both are provided",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host:     "https://api.basic-auth.local:6443",
					Username: "alice",
					Password: "secret",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.basic-auth.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				username:   opt[string]{enabled: true, value: "alice"},
				password:   opt[string]{enabled: true, value: "secret"},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch creates CA file when insecure is false and CAData is present",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.ca.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: false,
						CAData:   []byte("ca-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.ca.local:6443"},
				insecure:   opt[bool]{enabled: true, value: false},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathTempFile},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch does not create CA file when insecure is true even if CAData is present",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.ca-ignored.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
						CAData:   []byte("ca-data-ignored"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.ca-ignored.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch creates client cert file when CertData is present",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.cert.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
						CertData: []byte("cert-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.cert.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathTempFile},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch creates client key file when KeyData is present",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.key.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
						KeyData:  []byte("key-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.key.local:6443"},
				insecure:   opt[bool]{enabled: true, value: true},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathTempFile},
			},
		},
		{
			name:   "non-exec branch creates CA cert and key files when all tls data is present and insecure is false",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.full-tls.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: false,
						CAData:   []byte("ca-data"),
						CertData: []byte("cert-data"),
						KeyData:  []byte("key-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: false},
				apiServer:  opt[string]{enabled: true, value: "https://api.full-tls.local:6443"},
				insecure:   opt[bool]{enabled: true, value: false},
				kubeConfig: pathWant{enabled: true, kind: pathDevNull},
				caFile:     pathWant{enabled: true, kind: pathTempFile},
				certFile:   pathWant{enabled: true, kind: pathTempFile},
				keyFile:    pathWant{enabled: true, kind: pathTempFile},
			},
		},
		{
			name:   "exec branch selected when exec provider exists and bearer token is empty",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.exec.local:6443",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "aws",
						Args:       []string{"eks", "get-token"},
						APIVersion: "client.authentication.k8s.io/v1",
					},
					BearerToken: "",
				},
			},
			want: want{
				branch:     branchExec,
				err:        wantErr{want: false},
				kubeConfig: pathWant{enabled: true, kind: pathTempFile},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "exec branch preserves servername and auth provider configuration while still returning kubeconfig temp file",
			failAt: failNone,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.exec-authprovider.local:6443",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "kubelogin",
						Args:       []string{"get-token"},
						APIVersion: "client.authentication.k8s.io/v1beta1",
					},
					AuthProvider: &clientcmdapi.AuthProviderConfig{
						Name:   "gcp",
						Config: map[string]string{"scopes": "https://www.googleapis.com/auth/cloud-platform"},
					},
					BearerToken: "",
					TLSClientConfig: rest.TLSClientConfig{
						ServerName: "kubernetes.default.svc",
						Insecure:   false,
						CAFile:     "/etc/ssl/custom-ca.pem",
						CAData:     []byte("cluster-ca"),
					},
				},
			},
			want: want{
				branch:     branchExec,
				err:        wantErr{want: false},
				kubeConfig: pathWant{enabled: true, kind: pathTempFile},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch returns error when CA temp file creation fails",
			failAt: failCADataTempFile,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.fail-ca.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: false,
						CAData:   []byte("ca-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: true, contains: "temp"},
				kubeConfig: pathWant{enabled: true, kind: pathNil},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch returns error when cert temp file creation fails after CA file creation",
			failAt: failCertDataTempFile,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.fail-cert.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: false,
						CAData:   []byte("ca-data"),
						CertData: []byte("cert-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: true, contains: "temp"},
				kubeConfig: pathWant{enabled: true, kind: pathNil},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "non-exec branch returns error when key temp file creation fails after CA and cert creation",
			failAt: failKeyDataTempFile,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.fail-key.local:6443",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: false,
						CAData:   []byte("ca-data"),
						CertData: []byte("cert-data"),
						KeyData:  []byte("key-data"),
					},
				},
			},
			want: want{
				branch:     branchNonExec,
				err:        wantErr{want: true, contains: "temp"},
				kubeConfig: pathWant{enabled: true, kind: pathNil},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "exec branch returns error when kubeconfig serialization fails",
			failAt: failExecConfigSerialize,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.fail-serialize.local:6443",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "aws",
						Args:       []string{"eks", "get-token"},
						APIVersion: "client.authentication.k8s.io/v1",
					},
					BearerToken: "",
				},
			},
			want: want{
				branch:     branchExec,
				err:        wantErr{want: true, contains: "failed to write kubeconfig"},
				kubeConfig: pathWant{enabled: true, kind: pathNil},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
		{
			name:   "exec branch returns error when kubeconfig temp file creation fails",
			failAt: failExecConfigTempFile,
			client: &Client{
				RestConfig: rest.Config{
					Host: "https://api.fail-exec-tempfile.local:6443",
					ExecProvider: &clientcmdapi.ExecConfig{
						Command:    "aws",
						Args:       []string{"eks", "get-token"},
						APIVersion: "client.authentication.k8s.io/v1",
					},
					BearerToken: "",
				},
			},
			want: want{
				branch:     branchExec,
				err:        wantErr{want: true, contains: "failed to get kubeconfig file"},
				kubeConfig: pathWant{enabled: true, kind: pathNil},
				caFile:     pathWant{enabled: true, kind: pathNil},
				certFile:   pathWant{enabled: true, kind: pathNil},
				keyFile:    pathWant{enabled: true, kind: pathNil},
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// If you implemented DI for error injection, this should be a no-op for failNone.
			restore := setupFailPoint(t, test.failAt)
			defer restore()

			got, cleanup, err := test.client.setupKubeConfig()

			// contract: cleanup should always be non-nil (even on error paths)
			if cleanup == nil {
				t.Fatalf("cleanup func is nil")
			}
			defer cleanup()

			// error assertions
			if test.want.err.want {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if test.want.err.contains != "" && !strings.Contains(err.Error(), test.want.err.contains) {
					t.Fatalf("error %q does not contain %q", err.Error(), test.want.err.contains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("got nil kubeConfig")
			}

			// branch assertion
			actualBranch := branchExec
			if got.KubeConfig != nil && *got.KubeConfig == os.DevNull {
				actualBranch = branchNonExec
			}
			if actualBranch != test.want.branch {
				t.Fatalf("branch mismatch: got %q want %q", actualBranch, test.want.branch)
			}

			assertOptString := func(field string, gotPtr *string, exp opt[string]) {
				t.Helper()
				if !exp.enabled {
					return
				}
				if gotPtr == nil {
					t.Fatalf("%s is nil; want %q", field, exp.value)
				}
				if *gotPtr != exp.value {
					t.Fatalf("%s mismatch: got %q want %q", field, *gotPtr, exp.value)
				}
			}

			assertOptBool := func(field string, gotPtr *bool, exp opt[bool]) {
				t.Helper()
				if !exp.enabled {
					return
				}
				if gotPtr == nil {
					t.Fatalf("%s is nil; want %v", field, exp.value)
				}
				if *gotPtr != exp.value {
					t.Fatalf("%s mismatch: got %v want %v", field, *gotPtr, exp.value)
				}
			}

			assertPath := func(field string, gotPtr *string, exp pathWant) {
				t.Helper()
				if !exp.enabled || exp.kind == pathSkip {
					return
				}
				switch exp.kind {
				case pathNil:
					if gotPtr == nil {
						return
					}
					if *gotPtr != "" {
						t.Fatalf("%s expected nil/empty, got %q", field, *gotPtr)
					}
				case pathDevNull:
					if gotPtr == nil {
						t.Fatalf("%s expected %q, got nil", field, os.DevNull)
					}
					if *gotPtr != os.DevNull {
						t.Fatalf("%s expected %q, got %q", field, os.DevNull, *gotPtr)
					}
				case pathTempFile:
					if gotPtr == nil {
						t.Fatalf("%s expected temp file path, got nil", field)
					}
					if strings.TrimSpace(*gotPtr) == "" {
						t.Fatalf("%s expected non-empty temp file path", field)
					}
					if *gotPtr == os.DevNull {
						t.Fatalf("%s expected temp file path, got %q", field, os.DevNull)
					}
					if _, statErr := os.Stat(*gotPtr); statErr != nil {
						t.Fatalf("%s expected existing file at %q: %v", field, *gotPtr, statErr)
					}
				default:
					t.Fatalf("unknown path kind %q for field %s", exp.kind, field)
				}
			}

			assertOptString("APIServer", got.APIServer, test.want.apiServer)
			assertOptString("BearerToken", got.BearerToken, test.want.bearerToken)
			assertOptBool("Insecure", got.Insecure, test.want.insecure)
			assertOptString("Username", got.Username, test.want.username)
			assertOptString("Password", got.Password, test.want.password)

			assertPath("KubeConfig", got.KubeConfig, test.want.kubeConfig)
			assertPath("CAFile", got.CAFile, test.want.caFile)
			assertPath("CertFile", got.CertFile, test.want.certFile)
			assertPath("KeyFile", got.KeyFile, test.want.keyFile)
		})
	}
}
