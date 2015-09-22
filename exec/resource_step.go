package exec

import (
	"archive/tar"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
)

type resourceStep struct {
	StepMetadata StepMetadata

	SourceName SourceName

	Session resource.Session

	Delegate ResourceDelegate

	Tracker resource.Tracker
	Type    resource.ResourceType
	Tags    atc.Tags

	Action func(resource.Resource, ArtifactSource, VersionInfo) resource.VersionedSource

	PreviousStep Step
	Repository   *SourceRepository

	Resource        resource.Resource
	VersionedSource resource.VersionedSource

	exitStatus int
}

func (step resourceStep) Using(prev Step, repo *SourceRepository) Step {
	step.PreviousStep = prev
	step.Repository = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.Delegate.Failed,
	}
}

func (ras *resourceStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	trackedResource, err := ras.Tracker.Init(ras.StepMetadata, ras.Session, ras.Type, ras.Tags)
	if err != nil {
		return err
	}

	var versionInfo VersionInfo

	ras.PreviousStep.Result(&versionInfo)

	ras.Resource = trackedResource
	ras.VersionedSource = ras.Action(trackedResource, ras.Repository, versionInfo)

	err = ras.VersionedSource.Run(signals, ready)

	if err, ok := err.(resource.ErrResourceScriptFailed); ok {
		ras.exitStatus = err.ExitStatus
		ras.Delegate.Completed(ExitStatus(err.ExitStatus), nil)
		return nil
	}

	if err != nil {
		return err
	}

	if ras.SourceName != "" {
		ras.Repository.RegisterSource(ras.SourceName, ras)
	}

	ras.exitStatus = 0
	ras.Delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  ras.VersionedSource.Version(),
		Metadata: ras.VersionedSource.Metadata(),
	})

	return nil
}

func (ras *resourceStep) Release() {
	if ras.Resource != nil {
		ras.Resource.Release()
	}
}

func (ras *resourceStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = ras.exitStatus == 0
		return true
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
		return nil, FileNotFoundError{Path: path}
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}
