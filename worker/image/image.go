package image

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

const ImageMetadataFile = "metadata.json"

// ErrImageUnavailable is returned when a task's configured image resource
// has no versions.
var ErrImageUnavailable = errors.New("no versions of image available")

var ErrImageGetDidNotProduceVolume = errors.New("fetching the image did not produce a volume")

type image struct {
	logger                lager.Logger
	signals               <-chan os.Signal
	imageResource         atc.ImageResource
	workerID              worker.Identifier
	workerMetadata        worker.Metadata
	workerTags            atc.Tags
	teamID                int
	customTypes           atc.ResourceTypes
	tracker               resource.Tracker
	imageFetchingDelegate worker.ImageFetchingDelegate
	workerClient          worker.Client
	privileged            bool

	resourceFetcher resource.Fetcher
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

	resourceType := resource.ResourceType(i.imageResource.Type)

	resourceOptions := &imageResource{
		imageFetchingDelegate: i.imageFetchingDelegate,
		source:                i.imageResource.Source,
		version:               version,
		resourceType:          resourceType,
	}

	fetchSource, err := i.resourceFetcher.Fetch(
		i.logger.Session("init-image"),
		getSess,
		i.workerTags,
		i.teamID,
		i.customTypes,
		cacheID,
		resource.EmptyMetadata{},
		i.imageFetchingDelegate,
		resourceOptions,
		i.signals,
		make(chan struct{}),
	)
	if err != nil {
		i.logger.Debug("failed-to-fetch-image")
		return nil, nil, nil, err
	}

	vesionedSource := fetchSource.VersionedSource()
	volume := vesionedSource.Volume()
	if volume == nil {
		return nil, nil, nil, ErrImageGetDidNotProduceVolume
	}

	i.logger.Debug("created-volume-for-image", lager.Data{"handle": volume.Handle()})

	reader, err := vesionedSource.StreamOut(ImageMetadataFile)
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
		releaseFunc: func() { fetchSource.Release(nil) },
	}

	return volume, releasingReader, version, nil
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
		i.teamID,
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

type imageResource struct {
	imageFetchingDelegate worker.ImageFetchingDelegate
	source                atc.Source
	version               atc.Version
	resourceType          resource.ResourceType
}

func (d *imageResource) IOConfig() resource.IOConfig {
	return resource.IOConfig{
		Stderr: d.imageFetchingDelegate.Stderr(),
	}
}

func (ir *imageResource) Source() atc.Source {
	return ir.source
}

func (ir *imageResource) Params() atc.Params {
	return nil
}

func (ir *imageResource) Version() atc.Version {
	return ir.version
}

func (ir *imageResource) ResourceType() resource.ResourceType {
	return ir.resourceType
}

func (ir *imageResource) LockName(workerName string) (string, error) {
	id := &leaseID{
		Type:       ir.resourceType,
		Version:    ir.version,
		Source:     ir.source,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
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
