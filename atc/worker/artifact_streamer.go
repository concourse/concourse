package worker

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/runtime"
)

//go:generate counterfeiter . ArtifactStreamer

type ArtifactStreamer interface {
	StreamFileFromArtifact(context.Context, runtime.Artifact, string) (io.ReadCloser, error)
}

func NewArtifactStreamer(volumeFinder VolumeFinder, compression compression.Compression) ArtifactStreamer {
	return artifactStreamer{
		volumeFinder: volumeFinder,
		compression:  compression,
	}
}

type artifactStreamer struct {
	volumeFinder VolumeFinder
	compression  compression.Compression
}

func (a artifactStreamer) StreamFileFromArtifact(
	ctx context.Context,
	artifact runtime.Artifact,
	filePath string,
) (io.ReadCloser, error) {
	artifactVolume, found, err := a.volumeFinder.FindVolume(lagerctx.FromContext(ctx), 0, artifact.ID())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, baggageclaim.ErrVolumeNotFound
	}
	source := artifactSource{
		artifact:    artifact,
		volume:      artifactVolume,
		compression: a.compression,
	}
	return source.StreamFile(ctx, filePath)
}
