package oci

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"	
	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/client"
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
		return nil, err
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

