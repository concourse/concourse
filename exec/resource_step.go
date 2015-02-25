package exec

import (
	"archive/tar"
	"io"
	"os"

	"github.com/concourse/atc/resource"
)

type resourceStep struct {
	SessionID resource.SessionID

	Delegate ResourceDelegate

	Tracker resource.Tracker
	Type    resource.ResourceType

	Action func(resource.Resource, ArtifactSource) resource.VersionedSource

	ArtifactSource ArtifactSource

	Resource        resource.Resource
	VersionedSource resource.VersionedSource
}

func (step resourceStep) Using(source ArtifactSource) ArtifactSource {
	step.ArtifactSource = source

	return failureReporter{
		ArtifactSource: &step,
		ReportFailure:  step.Delegate.Failed,
	}
}

func (ras *resourceStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	resource, err := ras.Tracker.Init(ras.SessionID, ras.Type)
	if err != nil {
		return err
	}

	ras.Resource = resource
	ras.VersionedSource = ras.Action(resource, ras.ArtifactSource)

	err = ras.VersionedSource.Run(signals, ready)
	if err != nil {
		return err
	}

	ras.Delegate.Completed(VersionInfo{
		Version:  ras.VersionedSource.Version(),
		Metadata: ras.VersionedSource.Metadata(),
	})

	return nil
}

func (ras *resourceStep) Release() error {
	if ras.Resource != nil {
		return ras.Resource.Destroy()
	}

	return nil
}

func (ras *resourceStep) StreamTo(destination ArtifactDestination) error {
	out, err := ras.VersionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (ras *resourceStep) StreamFile(path string) (io.ReadCloser, error) {
	out, err := ras.VersionedSource.StreamOut(path)
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, ErrFileNotFound
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

func (ras *resourceStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *VersionInfo:
		*v = VersionInfo{
			Version:  ras.VersionedSource.Version(),
			Metadata: ras.VersionedSource.Metadata(),
		}
		return true

	default:
		return false
	}
}

type fileReadCloser struct {
	io.Reader
	io.Closer
}

type resourceSource struct {
	ArtifactSource
}

func (source resourceSource) StreamTo(dest resource.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(resource.ArtifactDestination(dest))
}
