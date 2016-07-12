package image

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

const FetchImageLeaseInterval = 5 * time.Second

const ImageMetadataFile = "metadata.json"

// ErrImageUnavailable is returned when a task's configured image resource
// has no versions.
var ErrImageUnavailable = errors.New("no versions of image available")

var ErrImageGetDidNotProduceVolume = errors.New("fetching the image did not produce a volume")

var ErrFailedToGetLease = errors.New("failed-to-get-lease")
var ErrInterrupted = errors.New("interrupted")

type image struct {
	logger                lager.Logger
	db                    LeaseDB
	signals               <-chan os.Signal
	imageResource         atc.ImageResource
	workerID              worker.Identifier
	workerMetadata        worker.Metadata
	workerTags            atc.Tags
	customTypes           atc.ResourceTypes
	tracker               resource.Tracker
	imageFetchingDelegate worker.ImageFetchingDelegate
	workerClient          worker.Client
	clock                 clock.Clock
	privileged            bool
}

func (i *image) Fetch() (worker.Volume, io.ReadCloser, atc.Version, error) {
	version, err := i.getLatestVersion()
	if err != nil {
		i.logger.Error("failed-to-get-latest-image-version", err)
		return nil, nil, nil, err
	}

	cacheID := resource.ResourceCacheIdentifier{
		Type:    resource.ResourceType(i.imageResource.Type),
		Version: version,
		Source:  i.imageResource.Source,
	}

	volumeID := cacheID.VolumeIdentifier()

	err = i.imageFetchingDelegate.ImageVersionDetermined(volumeID)
	if err != nil {
		return nil, nil, nil, err
	}

	getSess := resource.Session{
		ID:       i.workerID,
		Metadata: i.workerMetadata,
	}

	getSess.ID.Stage = db.ContainerStageGet
	getSess.ID.ImageResourceType = i.imageResource.Type
	getSess.ID.ImageResourceSource = i.imageResource.Source
	getSess.Metadata.Type = db.ContainerTypeGet
	getSess.Metadata.WorkingDirectory = ""
	getSess.Metadata.EnvironmentVariables = nil

	getResource, versionedSource, err := i.fetchWithLease(getSess, cacheID, version)
	if err != nil {
		i.logger.Debug("failed-to-fetch-image")
		return nil, nil, nil, err
	}

	volume, found := getResource.CacheVolume()
	if !found {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	volumeSpec := worker.VolumeSpec{
		Strategy: worker.ContainerRootFSStrategy{
			Parent: volume,
		},
		Privileged: i.privileged,
		TTL:        worker.ContainerTTL,
	}
	cowVolume, err := i.workerClient.CreateVolume(i.logger.Session("create-cow-volume"), volumeSpec)
	if err != nil {
		return nil, nil, nil, err
	}

	volume.Release(nil)

	reader, err := versionedSource.StreamOut(ImageMetadataFile)
	if err != nil {
		return nil, nil, nil, err
	}

	tarReader := tar.NewReader(reader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read file \"%s\" from tar", ImageMetadataFile)
	}

	releasingReader := &releasingReadCloser{
		Reader:      tarReader,
		Closer:      reader,
		releaseFunc: func() { getResource.Release(nil) },
	}

	return cowVolume, releasingReader, version, nil
}

func (i *image) getLatestVersion() (atc.Version, error) {
	checkSess := resource.Session{
		ID:       i.workerID,
		Metadata: i.workerMetadata,
	}

	checkSess.ID.Stage = db.ContainerStageCheck
	checkSess.ID.ImageResourceType = i.imageResource.Type
	checkSess.ID.ImageResourceSource = i.imageResource.Source
	checkSess.Metadata.Type = db.ContainerTypeCheck
	checkSess.Metadata.WorkingDirectory = ""
	checkSess.Metadata.EnvironmentVariables = nil

	checkingResource, err := i.tracker.Init(
		i.logger.Session("check-image"),
		resource.EmptyMetadata{},
		checkSess,
		resource.ResourceType(i.imageResource.Type),
		i.workerTags,
		i.customTypes,
		i.imageFetchingDelegate,
	)
	if err != nil {
		return nil, err
	}

	defer checkingResource.Release(nil)

	versions, err := checkingResource.Check(i.imageResource.Source, nil)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, ErrImageUnavailable
	}

	return versions[0], nil
}

type leaseID struct {
	Type       resource.ResourceType `json:"type"`
	Version    atc.Version           `json:"version"`
	Source     atc.Source            `json:"source"`
	WorkerName string                `json:"worker_name"`
}

func (i *image) fetchWithLease(
	getSess resource.Session,
	cacheID resource.ResourceCacheIdentifier,
	version atc.Version,
) (resource.Resource, resource.VersionedSource, error) {
	ticker := i.clock.NewTicker(FetchImageLeaseInterval)
	defer ticker.Stop()

	getResource, versionedSource, err := i.fetchImage(getSess, cacheID, version)
	if err != ErrFailedToGetLease {
		return getResource, versionedSource, err
	}

	for {
		select {
		case <-ticker.C():
			getResource, versionedSource, err := i.fetchImage(getSess, cacheID, version)
			if err != nil {
				if err == ErrFailedToGetLease {
					break
				}
				return nil, nil, err
			}

			return getResource, versionedSource, nil

		case <-i.signals:
			return nil, nil, ErrInterrupted
		}
	}
}

func (i *image) fetchImage(
	getSess resource.Session,
	cacheID resource.ResourceCacheIdentifier,
	version atc.Version,
) (resource.Resource, resource.VersionedSource, error) {
	resourceType := resource.ResourceType(i.imageResource.Type)

	getResource, cache, found, err := i.tracker.FindContainerForSession(
		i.logger.Session("find-container-for-image"),
		getSess,
	)
	if err != nil {
		i.logger.Error("failed-to-find-container-for-image", err)
		return nil, nil, err
	}

	if found {
		i.logger.Debug("found-container-for-image")
		return getResource, i.versionedResource(getResource, version), nil
	}

	choosenWorker, err := i.tracker.ChooseWorker(resourceType, i.workerTags, i.customTypes)
	if err != nil {
		i.logger.Error("no-workers-satisfying-spec", err)
		return nil, nil, err
	}

	leaseName, err := i.leaseName(leaseID{
		Type:       resourceType,
		Version:    version,
		Source:     i.imageResource.Source,
		WorkerName: choosenWorker.Name(),
	})
	if err != nil {
		i.logger.Error("failed-to-marshal-lease-id", err)
		return nil, nil, err
	}

	leaseLogger := i.logger.Session("lease-task", lager.Data{"lease-name": leaseName})
	leaseLogger.Info("tick")

	lease, leased, err := i.db.GetLease(leaseLogger, leaseName, FetchImageLeaseInterval)

	if err != nil {
		leaseLogger.Error("failed-to-get-lease", err)
		return nil, nil, ErrFailedToGetLease
	}

	if !leased {
		leaseLogger.Debug("did-not-get-lease")
		return nil, nil, ErrFailedToGetLease
	}

	defer lease.Break()

	i.logger.Debug("container-not-found")
	getResource, cache, err = i.tracker.InitWithCache(
		i.logger.Session("init-image"),
		resource.EmptyMetadata{},
		getSess,
		resourceType,
		i.workerTags,
		cacheID,
		i.customTypes,
		i.imageFetchingDelegate,
		choosenWorker,
	)
	if err != nil {
		leaseLogger.Error("failed-to-init-with-cache", err)
		return nil, nil, err
	}

	versionedSource := i.versionedResource(getResource, version)

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		leaseLogger.Error("failed-to-check-if-initialized", err)
		return nil, nil, err
	}

	if isInitialized {
		leaseLogger.Debug("cache-is-initiialized")
		return getResource, versionedSource, nil
	}

	leaseLogger.Debug("fetching-image")

	err = versionedSource.Run(i.signals, make(chan struct{}))
	if err != nil {
		leaseLogger.Error("failed-to-fetch-image", err, lager.Data{"lease-name": leaseName})
		return nil, nil, err
	}

	leaseLogger.Debug("initializing cache")
	err = cache.Initialize()
	if err != nil {
		leaseLogger.Error("failed-to-initialize-cache", err, lager.Data{"lease-name": leaseName})
		return nil, nil, err
	}

	return getResource, versionedSource, nil
}

func (i *image) leaseName(id leaseID) (string, error) {
	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return string(taskNameJSON), nil
}

func (i *image) versionedResource(getResource resource.Resource, version atc.Version) resource.VersionedSource {
	return getResource.Get(
		nil,
		resource.IOConfig{
			Stderr: i.imageFetchingDelegate.Stderr(),
		},
		i.imageResource.Source,
		nil,
		version,
		nil,
	)
}

type releasingReadCloser struct {
	io.Reader
	io.Closer
	releaseFunc func()
}

func (r *releasingReadCloser) Close() error {
	r.releaseFunc()
	return r.Closer.Close()
}
