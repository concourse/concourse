package libcontainerd

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

type testImageLayout struct {
	dir      string
	manifest ocispec.Descriptor
	config   ocispec.Descriptor
	layer    ocispec.Descriptor
}

func newTestImageLayout(t *testing.T) testImageLayout {
	l := testImageLayout{dir: t.TempDir()}

	l.config = l.writeBlob(t, ocispec.MediaTypeImageConfig, []byte(`{"os":"windows","architecture":"amd64"}`))
	l.layer = l.writeBlob(t, ocispec.MediaTypeImageLayerGzip, []byte("windows layer with its tar headers"))

	manifest, err := json.Marshal(ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    l.config,
		Layers:    []ocispec.Descriptor{l.layer},
	})
	require.NoError(t, err)

	l.manifest = l.writeBlob(t, ocispec.MediaTypeImageManifest, manifest)
	l.writeIndex(t, l.manifest)

	return l
}

func (l testImageLayout) writeBlob(t *testing.T, mediaType string, blob []byte) ocispec.Descriptor {
	desc := ocispec.Descriptor{MediaType: mediaType, Digest: digest.FromBytes(blob), Size: int64(len(blob))}

	path := filepath.Join(l.dir, ocispec.ImageBlobsDir, desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, blob, 0644))

	return desc
}

func (l testImageLayout) writeIndex(t *testing.T, manifests ...ocispec.Descriptor) {
	index, err := json.Marshal(ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: manifests,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(l.dir, ocispec.ImageIndexFile), index, 0644))
}

type testLabels map[digest.Digest]map[string]string

func (l testLabels) Get(d digest.Digest) (map[string]string, error) {
	return l[d], nil
}

func (l testLabels) Set(d digest.Digest, labels map[string]string) error {
	l[d] = labels
	return nil
}

func (l testLabels) Update(d digest.Digest, update map[string]string) (map[string]string, error) {
	labels := maps.Clone(l[d])
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range update {
		if v == "" {
			delete(labels, k)
		} else {
			labels[k] = v
		}
	}
	l[d] = labels
	return labels, nil
}

func newTestContentStore(t *testing.T) content.Store {
	store, err := local.NewLabeledStore(t.TempDir(), testLabels{})
	require.NoError(t, err)
	return store
}

func TestImportImageLayoutCopiesTheImageIntoTheContentStore(t *testing.T) {
	l := newTestImageLayout(t)

	store := newTestContentStore(t)

	target, err := importImageLayout(context.Background(), store, l.dir)
	require.NoError(t, err)
	require.Equal(t, l.manifest, target)

	for _, desc := range []ocispec.Descriptor{l.manifest, l.config, l.layer} {
		blob, err := content.ReadBlob(context.Background(), store, desc)
		require.NoError(t, err)
		require.Equal(t, desc.Digest, digest.FromBytes(blob))
	}
}

func TestImportImageLayoutKeepsTheWholeImageThroughGarbageCollection(t *testing.T) {
	l := newTestImageLayout(t)
	store := newTestContentStore(t)

	_, err := importImageLayout(context.Background(), store, l.dir)
	require.NoError(t, err)

	manifest, err := store.Info(context.Background(), l.manifest.Digest)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"containerd.io/gc.ref.content.config": l.config.Digest.String(),
		"containerd.io/gc.ref.content.l.0":    l.layer.Digest.String(),
	}, manifest.Labels)
}

func TestImportImageLayoutFailsWhenABlobIsMissing(t *testing.T) {
	l := newTestImageLayout(t)
	require.NoError(t, os.Remove(filepath.Join(l.dir, ocispec.ImageBlobsDir, l.layer.Digest.Algorithm().String(), l.layer.Digest.Encoded())))

	store := newTestContentStore(t)

	_, err := importImageLayout(context.Background(), store, l.dir)
	require.True(t, errdefs.IsNotFound(err), err)
}

func TestImportImageLayoutFailsWithoutExactlyOneImage(t *testing.T) {
	l := newTestImageLayout(t)
	l.writeIndex(t, l.manifest, l.manifest)

	store := newTestContentStore(t)

	_, err := importImageLayout(context.Background(), store, l.dir)
	require.Error(t, err)
}

func TestImportImageLayoutRejectsAMalformedDigest(t *testing.T) {
	l := newTestImageLayout(t)
	l.manifest.Digest = "../../../outside"
	l.writeIndex(t, l.manifest)

	store := newTestContentStore(t)

	_, err := importImageLayout(context.Background(), store, l.dir)
	require.ErrorIs(t, err, digest.ErrDigestInvalidFormat)
}

func TestImportImageLayoutFailsOnAnExtractedRootfs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "Files", "Windows"), 0755))

	store := newTestContentStore(t)

	_, err := importImageLayout(context.Background(), store, dir)
	require.ErrorIs(t, err, os.ErrNotExist)
}
