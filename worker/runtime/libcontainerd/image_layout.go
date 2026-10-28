package libcontainerd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// importImageLayout copies the image stored as an OCI image layout in dir
// into the content store, returning the descriptor of the image.
func importImageLayout(ctx context.Context, store content.Ingester, dir string) (ocispec.Descriptor, error) {
	indexJSON, err := os.ReadFile(filepath.Join(dir, ocispec.ImageIndexFile))
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	var index ocispec.Index
	err = json.Unmarshal(indexJSON, &index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("parse %s: %w", ocispec.ImageIndexFile, err)
	}

	if len(index.Manifests) != 1 {
		return ocispec.Descriptor{}, fmt.Errorf("image layout must hold one image, found %d", len(index.Manifests))
	}

	layout := imageLayout(dir)

	copyBlob := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		blob, err := layout.ReaderAt(ctx, desc)
		if err != nil {
			return nil, err
		}
		defer blob.Close()

		return nil, content.WriteBlob(ctx, store, desc.Digest.String(), content.NewReader(blob), desc)
	})

	err = images.Dispatch(ctx, images.Handlers(
		copyBlob,
		images.FilterPlatforms(images.ChildrenHandler(layout), platforms.Default()),
	), nil, index.Manifests[0])
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return index.Manifests[0], nil
}

// imageLayout provides the blobs of an OCI image layout directory.
type imageLayout string

func (dir imageLayout) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	err := desc.Digest.Validate()
	if err != nil {
		return nil, err
	}

	return local.OpenReader(filepath.Join(string(dir), ocispec.ImageBlobsDir, desc.Digest.Algorithm().String(), desc.Digest.Encoded()))
}
