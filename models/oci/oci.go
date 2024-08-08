package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/client"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
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
		defer os.RemoveAll(tmpDir)
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
func PushToOCIRegistry(dirPath, registryAdd, repositoryAdd, imageTag, username, password string) error {

	fs, fileErr := file.New(".")
	if fileErr != nil {
		return ErrWriteFile(fileErr)
	}

	ctx := context.Background()

	mediaType := "application/vnd.test.folder"
	fileNames := []string{dirPath}
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := fs.Add(ctx, name, mediaType, "")
		if err != nil {
			return ErrAddLayer(err)
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
	}

	// Pack the folder and tag the packed manifest
	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, packageErr := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1_RC4, artifactType, opts)
	if packageErr != nil {
		return ErrGettingLayer(packageErr)
	}

	if tagErr := fs.Tag(ctx, manifestDescriptor, imageTag); tagErr != nil {
		return ErrWriteFile(tagErr)
	}

	// Connect to a remote repository
	repo, connectErr := remote.NewRepository(registryAdd + "/" + repositoryAdd)
	if connectErr != nil {
		return ErrConnectingToRegistry(connectErr)
	}

	// Authenticate to the registry
	authErr := AuthToOCIRegistry(repo, registryAdd, username, password)
	if authErr != nil {
		return ErrAuthenticatingToRegistry(authErr)
	}

	_, pushErr := oras.Copy(ctx, fs, imageTag, repo, imageTag, oras.DefaultCopyOptions)
	if pushErr != nil {
		return ErrPushingPackage(pushErr)
	}

	return nil
}

// authentification to the public oci registry
// registryURL example : docker.io
func AuthToOCIRegistry(repo *remote.Repository, registryURI, username, password string) error {
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(registryURI, auth.Credential{
			Username: username,
			Password: password,
		}),
	}
	return nil
}

// function to pull images from the public oci repository
func PullFromOCIRegistry(dirPath, registryAdd, repositoryAdd, imageTag, username, password string) error {
	// Create a new file store
	fs, err := file.New(dirPath)
	if err != nil {
		return ErrFileNotFound(err, dirPath)
	}

	defer fs.Close()
	ctx := context.Background()

	// Connect to remote registry
	repo, connectErr := remote.NewRepository(registryAdd + "/" + repositoryAdd)
	if connectErr != nil {
		return ErrConnectingToRegistry(connectErr)
	}

	// Authenticate to the registry
	if username != "" && password != "" {
		authErr := AuthToOCIRegistry(repo, registryAdd, username, password)
		if authErr != nil {
			return ErrAuthenticatingToRegistry(authErr)
		}
	}

	_, pullErr := oras.Copy(ctx, repo, imageTag, fs, imageTag, oras.DefaultCopyOptions)
	if pullErr != nil {
		return ErrGettingImage(pullErr)
	}

	return nil
}
