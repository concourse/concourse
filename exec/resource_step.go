package exec

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type resourceStep struct {
	Logger lager.Logger

	WorkerClient worker.Client

	ResourceConfig atc.ResourceConfig
	Version        atc.Version
	Params         atc.Params

	StepMetadata StepMetadata

	SourceName SourceName

	Session resource.Session

	Delegate ResourceDelegate

	TrackerFactory TrackerFactory
	Type           resource.ResourceType
	Tags           atc.Tags

	Action func(resource.Resource, ArtifactSource, VersionInfo) resource.VersionedSource

	PreviousStep Step
	Repository   *SourceRepository

	Resource resource.Resource
	Volume   baggageclaim.Volume

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
	resourceSpec := worker.WorkerSpec{
		ResourceType: ras.ResourceConfig.Type,
		Tags:         ras.Tags,
	}

	chosenWorker, err := ras.WorkerClient.Satisfying(resourceSpec)
	if err != nil {
		return err
	}

	tracker := ras.TrackerFactory.TrackerFor(chosenWorker)

	mount := resource.VolumeMount{}

	var shouldRunGet bool

	vm, hasVM := chosenWorker.VolumeManager()

	if hasVM && ras.Version != nil {
		source, err := json.Marshal(ras.ResourceConfig.Source)
		if err != nil {
			return err
		}

		version, err := json.Marshal(ras.Version)
		if err != nil {
			return err
		}

		params, err := json.Marshal(ras.Params)
		if err != nil {
			return err
		}

		cachedVolumes, err := vm.FindVolumes(baggageclaim.VolumeProperties{
			"resource-type":    ras.ResourceConfig.Type,
			"resource-version": string(version),
			"resource-source":  string(source),
			"resource-params":  string(params),
			"initialized":      "yep",
		})
		if err != nil {
			return err
		}

		var cachedVolume baggageclaim.Volume
		if len(cachedVolumes) == 0 {
			shouldRunGet = true

			cachedVolume, err = vm.CreateEmptyVolume(baggageclaim.VolumeSpec{
				Properties: volume.Properties{ // TODO this should live under baggageclaim
					"resource-type":    ras.ResourceConfig.Type,
					"resource-version": string(version),
					"resource-source":  string(source),
					"resource-params":  string(params),
				},
				TTLInSeconds: 60 * 60 * 24,
			})
			if err != nil {
				return err
			}
		} else {
			cachedVolume = cachedVolumes[0]
		}

		ras.Volume = cachedVolume

		mount.Volume = cachedVolume
		mount.MountPath = resource.ResourcesDir("get")

		cachedVolume.Heartbeat(ras.Logger, time.Minute, clock.NewClock())
	} else {
		shouldRunGet = true
	}

	trackedResource, err := tracker.Init(ras.StepMetadata, ras.Session, ras.Type, ras.Tags, mount)
	if err != nil {
		return err
	}

	var versionInfo VersionInfo
	ras.PreviousStep.Result(&versionInfo)

	ras.Resource = trackedResource
	ras.VersionedSource = ras.Action(trackedResource, ras.Repository, versionInfo)

	if shouldRunGet {
		err = ras.VersionedSource.Run(signals, ready)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			ras.exitStatus = err.ExitStatus
			ras.Delegate.Completed(ExitStatus(err.ExitStatus), nil)
			return nil
		}

		if err != nil {
			return err
		}

		if mount.Volume != nil {
			mountedVolumes, err := ras.Resource.VolumeHandles()
			if err != nil {
				return err
			}

			if len(mountedVolumes) > 0 {
				// this is to handle the upgrade path where the container won't
				// initially have a volume mounted to it; the cache won't be populated,
				// so we should just ignore it
				err = vm.SetProperty(mount.Volume.Handle(), "initialized", "yep")
				if err != nil {
					return err
				}
			}
		}
	} else {
		fmt.Fprintf(ras.Delegate.Stdout(), "using version of resource found in cache\n")
		close(ready)
	}

	if ras.SourceName != "" {
		ras.Repository.RegisterSource(ras.SourceName, ras)
	}

	ras.exitStatus = 0
	if shouldRunGet {
		ras.Delegate.Completed(ExitStatus(0), &VersionInfo{
			Version:  ras.VersionedSource.Version(),
			Metadata: ras.VersionedSource.Metadata(),
		})
	} else {
		ras.Delegate.Completed(ExitStatus(0), &VersionInfo{
			Version: ras.Version,
		})
	}

	return nil
}

func (ras *resourceStep) Release() {
	if ras.Resource != nil {
		ras.Resource.Release()
	}

	if ras.Volume != nil {
		ras.Volume.Release()
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
