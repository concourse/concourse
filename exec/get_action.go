package exec

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type GetAction struct {
	Type         string
	Name         string
	Resource     string
	Source       atc.Source
	Params       atc.Params
	Version      atc.Version
	Tags         atc.Tags
	RootFSSource RootFSSource
	Outputs      []string

	result *atc.VersionInfo

	// TODO: can we remove these dependencies?
	imageFetchingDelegate ImageFetchingDelegate
	resourceFetcher       resource.Fetcher
	teamID                int
	containerMetadata     db.ContainerMetadata
	resourceInstance      resource.ResourceInstance
	stepMetadata          StepMetadata

	// TODO: remove after all actions are introduced
	resourceTypes atc.VersionedResourceTypes
}

func (action *GetAction) Run(
	logger lager.Logger,
	repository *worker.ArtifactRepository,

	// TODO: consider passing these as context
	signals <-chan os.Signal,
	ready chan<- struct{},
) error {
	// TODO: can we remove resource definition?
	resourceDefinition := &getResource{
		source:                action.Source,
		resourceType:          resource.ResourceType(action.Type),
		imageFetchingDelegate: action.imageFetchingDelegate,
		params:                action.Params,
		version:               action.Version,
	}

	versionedSource, err := action.resourceFetcher.Fetch(
		logger,
		resource.Session{
			Metadata: action.containerMetadata,
		},
		action.Tags,
		action.teamID,
		action.resourceTypes,
		action.resourceInstance,
		action.stepMetadata,
		action.imageFetchingDelegate,
		resourceDefinition,
		signals,
		ready,
	)

	if err != nil {
		logger.Error("failed-to-init-with-cache", err)
		return err
	}

	for _, outputName := range action.Outputs {
		repository.RegisterSource(worker.ArtifactName(outputName), &getArtifactSource{
			logger:           logger,
			resourceInstance: action.resourceInstance,
			versionedSource:  versionedSource,
		})
	}

	action.result = &atc.VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}

	return nil
}

type getArtifactSource struct {
	logger           lager.Logger
	resourceInstance resource.ResourceInstance
	versionedSource  resource.VersionedSource
}

// VolumeOn locates the cache for the GetStep's resource and version on the
// given worker.
func (s *getArtifactSource) VolumeOn(worker worker.Worker) (worker.Volume, bool, error) {
	return s.resourceInstance.FindOn(s.logger.Session("volume-on"), worker)
}

// StreamTo streams the resource's data to the destination.
func (s *getArtifactSource) StreamTo(destination worker.ArtifactDestination) error {
	out, err := s.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

// StreamFile streams a single file out of the resource.
func (s *getArtifactSource) StreamFile(path string) (io.ReadCloser, error) {
	out, err := s.versionedSource.StreamOut(path)
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

type getResource struct {
	imageFetchingDelegate ImageFetchingDelegate
	resourceType          resource.ResourceType
	source                atc.Source
	params                atc.Params
	version               atc.Version
}

func (d *getResource) IOConfig() resource.IOConfig {
	return resource.IOConfig{
		Stdout: d.imageFetchingDelegate.Stdout(),
		Stderr: d.imageFetchingDelegate.Stderr(),
	}
}

func (d *getResource) Source() atc.Source {
	return d.source
}

func (d *getResource) Params() atc.Params {
	return d.params
}

func (d *getResource) Version() atc.Version {
	return d.version
}

func (d *getResource) ResourceType() resource.ResourceType {
	return d.resourceType
}

func (d *getResource) LockName(workerName string) (string, error) {
	id := &getStepLockID{
		Type:       d.resourceType,
		Version:    d.version,
		Source:     d.source,
		Params:     d.params,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}

type getStepLockID struct {
	Type       resource.ResourceType `json:"type"`
	Version    atc.Version           `json:"version"`
	Source     atc.Source            `json:"source"`
	Params     atc.Params            `json:"params"`
	WorkerName string                `json:"worker_name"`
}
