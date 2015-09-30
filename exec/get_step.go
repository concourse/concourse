package exec

import (
	"archive/tar"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

const resourceTTLInSeconds = 60 * 60 * 24

type getStep struct {
	logger         lager.Logger
	sourceName     SourceName
	workerPool     worker.Client
	resourceConfig atc.ResourceConfig
	version        atc.Version
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
	trackerFactory TrackerFactory

	repository *SourceRepository

	resource resource.Resource

	versionedSource resource.VersionedSource

	exitStatus int
}

func newGetStep(
	logger lager.Logger,
	sourceName SourceName,
	workerPool worker.Client,
	resourceConfig atc.ResourceConfig,
	version atc.Version,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate ResourceDelegate,
	trackerFactory TrackerFactory,
) getStep {
	return getStep{
		logger:         logger,
		sourceName:     sourceName,
		workerPool:     workerPool,
		resourceConfig: resourceConfig,
		version:        version,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		trackerFactory: trackerFactory,
	}
}

func (step getStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

func (step *getStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	resourceSpec := worker.WorkerSpec{
		ResourceType: step.resourceConfig.Type,
		Tags:         step.tags,
	}

	chosenWorker, err := step.workerPool.Satisfying(resourceSpec)
	if err != nil {
		return err
	}

	tracker := step.trackerFactory.TrackerFor(chosenWorker)

	mount := resource.VolumeMount{}

	var shouldRunGet bool

	vm, hasVM := chosenWorker.VolumeManager()

	var cachedVolume baggageclaim.Volume
	if hasVM {
		var workerAlreadyHasCache bool
		cachedVolume, workerAlreadyHasCache, err = step.VolumeOn(chosenWorker)
		if err != nil {
			return err
		}

		if workerAlreadyHasCache {
			step.logger.Info("found-cache", lager.Data{"handle": cachedVolume.Handle()})
		} else {
			step.logger.Debug("no-cache-found")

			shouldRunGet = true

			cachedVolume, err = vm.CreateVolume(step.logger, baggageclaim.VolumeSpec{
				Properties:   step.volumeProperties(),
				TTLInSeconds: resourceTTLInSeconds,
				Privileged:   true,
			})
			if err != nil {
				return err
			}

			step.logger.Info("initializing-cache", lager.Data{"handle": cachedVolume.Handle()})
		}

		mount.Volume = cachedVolume
		mount.MountPath = resource.ResourcesDir("get")
	} else {
		shouldRunGet = true
	}

	trackedResource, err := tracker.Init(
		step.logger,
		step.stepMetadata,
		step.session,
		resource.ResourceType(step.resourceConfig.Type),
		step.tags,
		mount,
	)
	if err != nil {
		return err
	}

	realCachedVolume, hasCachedVolume, err := trackedResource.CacheVolume()
	if err != nil {
		return err
	}

	// release our volume now that the container will keep the correct one alive
	// (which may be a different volume in the event that we've restarted and
	// reattached)
	if mount.Volume != nil {
		mount.Volume.Release()
	}

	step.resource = trackedResource

	step.versionedSource = step.resource.Get(
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		step.version,
	)

	if shouldRunGet {
		err = step.versionedSource.Run(signals, ready)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			step.exitStatus = err.ExitStatus
			step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
			return nil
		}

		if err != nil {
			return err
		}

		if hasCachedVolume {
			step.logger.Info("cache-initialized")

			err = realCachedVolume.SetProperty("initialized", "yep")
			if err != nil {
				return err
			}
		} else {
			// this is to handle the upgrade path where the container won't
			// initially have a volume mounted to it; the cache won't be populated,
			// so we should just ignore it
			step.logger.Info("ignoring-unpopulated-cache")
		}
	} else {
		fmt.Fprintf(step.delegate.Stdout(), "using version of resource found in cache\n")
		close(ready)
	}

	step.repository.RegisterSource(step.sourceName, step)

	step.exitStatus = 0
	if shouldRunGet {
		step.delegate.Completed(ExitStatus(0), &VersionInfo{
			Version:  step.versionedSource.Version(),
			Metadata: step.versionedSource.Metadata(),
		})
	} else {
		step.delegate.Completed(ExitStatus(0), &VersionInfo{
			Version: step.version,
		})
	}

	return nil
}

func (step *getStep) Release() {
	if step.resource != nil {
		step.resource.Release()
	}
}

func (step *getStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = step.exitStatus == 0
		return true
	case *VersionInfo:
		*v = VersionInfo{
			Version:  step.versionedSource.Version(),
			Metadata: step.versionedSource.Metadata(),
		}
		return true

	default:
		return false
	}
}

func (step *getStep) VolumeOn(worker worker.Worker) (baggageclaim.Volume, bool, error) {
	vm, hasVM := worker.VolumeManager()

	if !hasVM {
		return nil, false, nil
	}

	foundVolumes, err := vm.ListVolumes(step.logger, withInitialized(step.volumeProperties()))
	if err != nil {
		return nil, false, err
	}

	if len(foundVolumes) == 0 {
		return nil, false, nil
	} else {
		return selectLowestAlphabeticalVolume(foundVolumes), true, nil
	}
}

func (step *getStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *getStep) StreamFile(path string) (io.ReadCloser, error) {
	out, err := step.versionedSource.StreamOut(path)
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

func (step *getStep) volumeProperties() baggageclaim.VolumeProperties {
	source, _ := json.Marshal(step.resourceConfig.Source)

	version, _ := json.Marshal(step.version)

	params, _ := json.Marshal(step.params)

	return baggageclaim.VolumeProperties{
		"resource-type":    step.resourceConfig.Type,
		"resource-version": string(version),
		"resource-source":  shastr(source),
		"resource-params":  shastr(params),
	}
}

func withInitialized(props baggageclaim.VolumeProperties) baggageclaim.VolumeProperties {
	newProps := baggageclaim.VolumeProperties{}
	for k, v := range props {
		newProps[k] = v
	}

	newProps["initialized"] = "yep"

	return newProps
}

func shastr(b []byte) string {
	return fmt.Sprintf("%x", sha512.Sum512(b))
}

func selectLowestAlphabeticalVolume(volumes []baggageclaim.Volume) baggageclaim.Volume {
	var lowestVolume baggageclaim.Volume

	for _, v := range volumes {
		if lowestVolume == nil {
			lowestVolume = v
		} else if v.Handle() < lowestVolume.Handle() {
			lowestVolume = v
		}
	}

	for _, v := range volumes {
		if v != lowestVolume {
			v.Release()

			// setting TTL here is best-effort; don't worry about failure
			_ = v.SetTTL(60)
		}
	}

	return lowestVolume
}
