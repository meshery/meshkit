package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	orasremote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// LayerType is an enumeration of the supported layer types
// when pushing an image.
type LayerType string

const (
	// LayerTypeTarball produces a layer that contains a gzipped archive
	LayerTypeTarball LayerType = "tarball"
	// LayerTypeStatic produces a layer that contains the contents of a
	// file without any compression.
	LayerTypeStatic LayerType = "static"
)

// BuildOptions are options for configuring the Push operation.
type BuildOptions struct {
	layerType LayerType
	layerOpts layerOptions
	meta      client.Metadata
}

// layerOptions are options for configuring a layer.
type layerOptions struct {
	mediaTypeExt string
	ignorePaths  []string
}

// BuildOption is a function for configuring BuildOptions.
type BuildOption func(o *BuildOptions)

// Builds OCI Img for the artifacts in the given path. Returns v1.Image manifest.
func BuildImage(sourcePath string, opts ...BuildOption) (gcrv1.Image, error) {
	o := &BuildOptions{
		layerType: LayerTypeTarball,
	}

	for _, opt := range opts {
		opt(o)
	}

	layer, err := createLayer(sourcePath, o.layerType, o.layerOpts)
	if err != nil {
		return nil, ErrCreateLayer(err)
	}

	if o.meta.Created == "" {
		ct := time.Now().UTC()
		o.meta.Created = ct.Format(time.RFC3339)
	}

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)
	img = mutate.Annotations(img, o.meta.ToAnnotations()).(gcrv1.Image)

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, ErrAppendingLayer(err)
	}

	return img, nil
}

// createLayer creates a layer depending on the layerType.
func createLayer(path string, layerType LayerType, opts layerOptions) (gcrv1.Layer, error) {
	switch layerType {
	case LayerTypeTarball:
		var ociMediaType = oci.CanonicalContentMediaType
		var tmpDir string
		tmpDir, err := os.MkdirTemp("", "oci")
		if err != nil {
			return nil, err
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()
		tmpFile := filepath.Join(tmpDir, "artifact.tgz")
		defaultOpts := client.DefaultOptions()
		ociClient := client.NewClient(defaultOpts)
		if err := ociClient.Build(tmpFile, path, opts.ignorePaths); err != nil {
			return nil, err
		}
		return tarball.LayerFromFile(tmpFile, tarball.WithMediaType(ociMediaType), tarball.WithCompressedCaching)
	case LayerTypeStatic:
		var ociMediaType = getLayerMediaType(opts.mediaTypeExt)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, ErrReadingFile(err)
		}
		return static.NewLayer(content, ociMediaType), nil
	default:
		return nil, ErrUnSupportedLayerType(fmt.Errorf("unsupported layer type: '%s'", layerType))
	}
}

func getLayerMediaType(extension string) types.MediaType {
	if extension == "" {
		return oci.CanonicalMediaTypePrefix
	}
	return types.MediaType(fmt.Sprintf("%s.%s", oci.CanonicalMediaTypePrefix, extension))
}

// function to pull models from any OCI-compatible repository
func PushToOCIRegistry(img gcrv1.Image, layoutDirPath, registryAdd, repositoryAdd, imageTag, username, password string) error {
	p, writeErr := layout.Write(layoutDirPath, empty.Index)
	if writeErr != nil {
		return ErrWriteFile(writeErr)
	}

	if appendErr := p.AppendImage(img); appendErr != nil {
		return ErrAppendImageToLayout(appendErr)
	}

	ref, parseErr := name.ParseReference(fmt.Sprintf("%s/%s:%s", registryAdd, repositoryAdd, imageTag))
	if parseErr != nil {
		return ErrTaggingPackage(parseErr)
	}

	if pushErr := gcrremote.Write(ref, img, AuthToOCIRegistry(username, password)); pushErr != nil {
		return ErrPushingPackage(pushErr)
	}

	return nil
}

func AuthToOCIRegistry(username, password string) gcrremote.Option {
	return gcrremote.WithAuth(&authn.Basic{
		Username: username,
		Password: password,
	})
}

// function to pull images from the public oci repository
func PullFromOCIRegistry(dirPath, registryAdd, repositoryAdd, imageTag, username, password string) error {
	// Create a new file store
	fs, err := file.New(dirPath)
	if err != nil {
		return ErrFileNotFound(err, dirPath)
	}

	defer func() { _ = fs.Close() }()
	ctx := context.Background()

	// Connect to remote registry
	repo, connectErr := orasremote.NewRepository(registryAdd + "/" + repositoryAdd)
	if connectErr != nil {
		return ErrConnectingToRegistry(connectErr)
	}

	// Authenticate to the registry
	if username != "" && password != "" {
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(registryAdd, auth.Credential{
				Username: username,
				Password: password,
			}),
		}
	}

	_, pullErr := oras.Copy(ctx, repo, imageTag, fs, imageTag, oras.DefaultCopyOptions)
	if pullErr != nil {
		return ErrGettingImage(pullErr)
	}

	return nil
}
