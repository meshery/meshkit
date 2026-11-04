package oci

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/stretchr/testify/require"
)

func assertErrorCode(t *testing.T, err error, expected string) {
	t.Helper()

	meshkitErr, ok := err.(*meshkiterrors.Error)
	if !ok {
		t.Fatalf("expected *meshkiterrors.Error, got %T: %v", err, err)
	}
	if meshkitErr.Code != expected {
		t.Errorf("error code = %q, want %q", meshkitErr.Code, expected)
	}
}

func TestAuthToOCIRegistry_SendsBasicAuth(t *testing.T) {
	var gotAuth string

	regHandler := registry.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			gotAuth = auth
		}
		regHandler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	refStr := fmt.Sprintf("%s/test/repo:latest", host)
	ref, err := name.ParseReference(refStr)
	require.NoError(t, err)

	pushErr := remote.Write(ref, img, AuthToOCIRegistry("test-user", "test-password"))
	require.NoError(t, pushErr)

	require.NotEmpty(t, gotAuth, "expected an Authorization header to be sent")
	require.True(t, strings.HasPrefix(gotAuth, "Basic "), "expected Basic auth scheme, got: %s", gotAuth)

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(gotAuth, "Basic "))
	require.NoError(t, err)
	require.Equal(t, "test-user:test-password", string(decoded))
}

func TestPushToOCIRegistry(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	src := filepath.Join(t.TempDir(), "model.json")
	require.NoError(t, os.WriteFile(src, []byte(`{"name":"test-model"}`), 0600))

	img, err := BuildImage(src, func(o *BuildOptions) {
		o.layerType = LayerTypeStatic
	})
	require.NoError(t, err)

	layoutDir := t.TempDir()

	err = PushToOCIRegistry(img, layoutDir, host, "test/repo", "latest", "user", "pass")
	require.NoError(t, err)

	ref, err := name.ParseReference(fmt.Sprintf("%s/test/repo:latest", host))
	require.NoError(t, err)

	remoteImg, err := remote.Image(ref)
	require.NoError(t, err)

	wantDigest, err := img.Digest()
	require.NoError(t, err)
	gotDigest, err := remoteImg.Digest()
	require.NoError(t, err)
	require.Equal(t, wantDigest, gotDigest)

	p, err := layout.FromPath(layoutDir)
	require.NoError(t, err)
	idx, err := p.ImageIndex()
	require.NoError(t, err)
	idxManifest, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, idxManifest.Manifests, 1)
	require.Equal(t, wantDigest, idxManifest.Manifests[0].Digest)
}

func TestPushToOCIRegistry_Errors(t *testing.T) {
	src := filepath.Join(t.TempDir(), "model.json")
	require.NoError(t, os.WriteFile(src, []byte(`{}`), 0600))
	img, err := BuildImage(src, func(o *BuildOptions) {
		o.layerType = LayerTypeStatic
	})
	require.NoError(t, err)

	deny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer deny.Close()

	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0600))

	tests := []struct {
		name      string
		layoutDir string
		registry  string
		tag       string
		wantCode  string
	}{
		{"layout no escribible", filepath.Join(blocker, "sub"), "localhost:5000", "latest", ErrWriteFilesCode},
		{"tag invalido", t.TempDir(), "localhost:5000", "tag invalido", ErrTaggingPackageCode},
		{"registry rechaza el push", t.TempDir(), strings.TrimPrefix(deny.URL, "http://"), "latest", ErrPushingPackageCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PushToOCIRegistry(img, tt.layoutDir, tt.registry, "test/repo", tt.tag, "u", "p")
			require.Error(t, err)
			assertErrorCode(t, err, tt.wantCode)
		})
	}
}
