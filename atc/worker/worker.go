// Package worker will eventually evolve to becoming a concrete implementation of a runtime
// As such, Concourse core shouldn't depend on abstractions defined in this package or its child packages
// General Runtime abstractions will be ported over to the Runtime package
// The Client interface is the main interface that is consumed by Concourse core that will be shifted to the Runtime package
package worker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/cppforlife/go-semi-semantic/version"
	"golang.org/x/sync/errgroup"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/tracing"
)

const userPropertyName = "user"

var ResourceConfigCheckSessionExpiredError = errors.New("no db container was found for owner")

//go:generate counterfeiter . Worker

type Worker interface {
	BuildContainers() int

	Description() string
	Name() string
	ResourceTypes() []atc.WorkerResourceType
	Tags() atc.Tags
	Uptime() time.Duration
	IsOwnedByTeam() bool
	Ephemeral() bool
	IsVersionCompatible(lager.Logger, version.Version) bool
	Satisfies(lager.Logger, WorkerSpec) bool
	FindContainerByHandle(lager.Logger, int, string) (Container, bool, error)

	FindOrCreateContainer(
		context.Context,
		lager.Logger,
		ImageFetchingDelegate,
		db.ContainerOwner,
		db.ContainerMetadata,
		ContainerSpec,
		atc.VersionedResourceTypes,
	) (Container, error)

	FindVolumeForResourceCache(logger lager.Logger, resourceCache db.UsedResourceCache) (Volume, bool, error)
	FindResourceCacheForVolume(volume Volume) (db.UsedResourceCache, bool, error)
	FindVolumeForTaskCache(lager.Logger, int, int, string, string) (Volume, bool, error)
	Fetch(
		context.Context,
		lager.Logger,
		db.ContainerMetadata,
		Worker,
		ContainerSpec,
		runtime.ProcessSpec,
		resource.Resource,
		db.ContainerOwner,
		ImageFetcherSpec,
		db.UsedResourceCache,
		string,
	) (GetResult, Volume, error)

	CertsVolume(lager.Logger) (volume Volume, found bool, err error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)
	CreateVolume(logger lager.Logger, spec VolumeSpec, teamID int, volumeType db.VolumeType) (Volume, error)

	GardenClient() gclient.Client
	ActiveTasks() (int, error)
	IncreaseActiveTasks() error
	DecreaseActiveTasks() error
}

type gardenWorker struct {
	gardenClient         gclient.Client
	volumeClient         VolumeClient
	imageFactory         ImageFactory
	resourceCacheFactory db.ResourceCacheFactory
	Fetcher
	dbWorker        db.Worker
	buildContainers int
	helper          workerHelper
	policyChecker   *policy.Checker
}

// NewGardenWorker constructs a Worker using the gardenWorker runtime implementation and allows container and volume
// creation on a specific Garden worker.
// A Garden Worker is comprised of: db.Worker, garden Client, container provider, and a volume client
func NewGardenWorker(
	gardenClient gclient.Client,
	volumeRepository db.VolumeRepository,
	volumeClient VolumeClient,
	imageFactory ImageFactory,
	fetcher Fetcher,
	dbTeamFactory db.TeamFactory,
	dbWorker db.Worker,
	resourceCacheFactory db.ResourceCacheFactory,
	numBuildContainers int,
	policyChecker *policy.Checker,
	// TODO: numBuildContainers is only needed for placement strategy but this
	// method is called in ContainerProvider.FindOrCreateContainer as well and
	// hence we pass in 0 values for numBuildContainers everywhere.
) Worker {
	workerHelper := workerHelper{
		gardenClient:  gardenClient,
		volumeClient:  volumeClient,
		volumeRepo:    volumeRepository,
		dbTeamFactory: dbTeamFactory,
		dbWorker:      dbWorker,
	}

	return &gardenWorker{
		gardenClient:         gardenClient,
		volumeClient:         volumeClient,
		imageFactory:         imageFactory,
		Fetcher:              fetcher,
		dbWorker:             dbWorker,
		resourceCacheFactory: resourceCacheFactory,
		buildContainers:      numBuildContainers,
		helper:               workerHelper,
		policyChecker:        policyChecker,
	}
}

func (worker *gardenWorker) GardenClient() gclient.Client {
	return worker.gardenClient
}

func (worker *gardenWorker) IsVersionCompatible(logger lager.Logger, comparedVersion version.Version) bool {
	workerVersion := worker.dbWorker.Version()
	logger = logger.Session("check-version", lager.Data{
		"want-worker-version": comparedVersion.String(),
		"have-worker-version": workerVersion,
	})

	if workerVersion == nil {
		logger.Info("empty-worker-version")
		return false
	}

	v, err := version.NewVersionFromString(*workerVersion)
	if err != nil {
		logger.Error("failed-to-parse-version", err)
		return false
	}

	switch v.Release.Compare(comparedVersion.Release) {
	case 0:
		return true
	case -1:
		return false
	default:
		if v.Release.Components[0].Compare(comparedVersion.Release.Components[0]) == 0 {
			return true
		}

		return false
	}
}

func (worker *gardenWorker) FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool) {
	for _, rt := range worker.dbWorker.ResourceTypes() {
		if path == rt.Image {
			return rt, true
		}
	}

	return atc.WorkerResourceType{}, false
}

func (worker *gardenWorker) FindVolumeForResourceCache(logger lager.Logger, resourceCache db.UsedResourceCache) (Volume, bool, error) {
	return worker.volumeClient.FindVolumeForResourceCache(logger, resourceCache)
}

func (worker *gardenWorker) FindResourceCacheForVolume(volume Volume) (db.UsedResourceCache, bool, error) {
	if volume.GetResourceCacheID() != 0 {
		return worker.resourceCacheFactory.FindResourceCacheByID(volume.GetResourceCacheID())
	} else {
		return nil, false, nil
	}

}

func (worker *gardenWorker) FindVolumeForTaskCache(logger lager.Logger, teamID int, jobID int, stepName string, path string) (Volume, bool, error) {
	return worker.volumeClient.FindVolumeForTaskCache(logger, teamID, jobID, stepName, path)
}

func (worker *gardenWorker) CertsVolume(logger lager.Logger) (Volume, bool, error) {
	return worker.volumeClient.FindOrCreateVolumeForResourceCerts(logger.Session("find-or-create"))
}

func (worker *gardenWorker) CreateVolume(logger lager.Logger, spec VolumeSpec, teamID int, volumeType db.VolumeType) (Volume, error) {
	return worker.volumeClient.CreateVolume(logger.Session("find-or-create"), spec, teamID, worker.dbWorker.Name(), volumeType)
}

func (worker *gardenWorker) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	return worker.volumeClient.LookupVolume(logger, handle)
}

func (worker *gardenWorker) imagePolicyCheck(
	ctx context.Context,
	delegate ImageFetchingDelegate,
	metadata db.ContainerMetadata,
	containerSpec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
) (bool, error) {
	if worker.policyChecker == nil {
		return true, nil
	}

	// Actions in skip list will not go through policy check.
	if !worker.policyChecker.ShouldCheckAction(policy.ActionUseImage) {
		return true, nil
	}

	imageSpec := containerSpec.ImageSpec

	imageInfo := map[string]interface{}{
		"privileged": imageSpec.Privileged,
	}

	if imageSpec.ImageResource != nil {
		imageInfo["image_type"] = imageSpec.ImageResource.Type
		imageInfo["image_source"] = imageSpec.ImageResource.Source
	} else if imageSpec.ResourceType != "" {
		for _, rt := range resourceTypes {
			if rt.Name == imageSpec.ResourceType {
				imageInfo["image_type"] = rt.Type
				imageInfo["image_source"] = rt.Source
			}
		}

		// If resource type not found, then it should be a built-in resource
		// type, and could skip policy check.
		if _, ok := imageInfo["image_type"]; !ok {
			return true, nil
		}
	} else {
		// Ignore other images as policy checker cannot do much on them.
		return true, nil
	}

	if originalSource, ok := imageInfo["image_source"].(atc.Source); ok {
		redactedSource, err := delegate.RedactImageSource(originalSource)
		if err != nil {
			return false, err
		}
		imageInfo["image_source"] = redactedSource
	}

	teamName, pipelineName := policy.TeamAndPipelineFromContext(ctx)
	input := policy.PolicyCheckInput{
		Action:   policy.ActionUseImage,
		Team:     teamName,
		Pipeline: pipelineName,
		Data:     imageInfo,
	}

	return worker.policyChecker.Check(input)
}

func (worker *gardenWorker) FindOrCreateContainer(
	ctx context.Context,
	logger lager.Logger,
	delegate ImageFetchingDelegate,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
) (Container, error) {

	var (
		gardenContainer   gclient.Container
		createdContainer  db.CreatedContainer
		creatingContainer db.CreatingContainer
		containerHandle   string
		err               error
	)

	pass, err := worker.imagePolicyCheck(ctx, delegate, metadata, containerSpec, resourceTypes)
	if err != nil {
		return nil, err
	}
	if !pass {
		return nil, policy.PolicyCheckNotPass{}
	}

	// ensure either creatingContainer or createdContainer exists
	creatingContainer, createdContainer, err = worker.dbWorker.FindContainer(owner)
	if err != nil {
		return nil, err
	}

	if creatingContainer != nil {
		containerHandle = creatingContainer.Handle()
	} else if createdContainer != nil {
		containerHandle = createdContainer.Handle()
	} else {
		logger.Debug("creating-container-in-db")
		creatingContainer, err = worker.dbWorker.CreateContainer(
			owner,
			metadata,
		)
		if err != nil {
			logger.Error("failed-to-create-container-in-db", err)
			if _, ok := err.(db.ContainerOwnerDisappearedError); ok {
				return nil, ResourceConfigCheckSessionExpiredError
			}

			return nil, err
		}
		logger.Debug("created-creating-container-in-db")
		containerHandle = creatingContainer.Handle()
	}

	logger = logger.WithData(lager.Data{"container": containerHandle})

	gardenContainer, err = worker.gardenClient.Lookup(containerHandle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); !ok {
			logger.Error("failed-to-lookup-creating-container-in-garden", err)
			return nil, err
		}
	}

	// if createdContainer exists, gardenContainer should also exist
	if createdContainer != nil {
		logger = logger.WithData(lager.Data{"container": containerHandle})
		logger.Debug("found-created-container-in-db")

		if gardenContainer == nil {
			return nil, garden.ContainerNotFoundError{Handle: containerHandle}
		}
		return worker.helper.constructGardenWorkerContainer(
			logger,
			createdContainer,
			gardenContainer,
		)
	}

	// we now have a creatingContainer. If a gardenContainer does not exist, we
	// will create one. If it does exist, we will transition the creatingContainer
	// to created and return a worker.Container
	if gardenContainer == nil {
		fetchedImage, err := worker.fetchImageForContainer(
			ctx,
			logger,
			containerSpec.ImageSpec,
			containerSpec.TeamID,
			delegate,
			resourceTypes,
			creatingContainer,
		)
		if err != nil {
			creatingContainer.Failed()
			logger.Error("failed-to-fetch-image-for-container", err)
			return nil, err
		}

		volumeMounts, err := worker.createVolumes(ctx, logger, fetchedImage.Privileged, creatingContainer, containerSpec)
		if err != nil {
			creatingContainer.Failed()
			logger.Error("failed-to-create-volume-mounts-for-container", err)
			return nil, err
		}
		bindMounts, err := worker.getBindMounts(volumeMounts, containerSpec.BindMounts)
		if err != nil {
			creatingContainer.Failed()
			logger.Error("failed-to-create-bind-mounts-for-container", err)
			return nil, err
		}

		logger.Debug("creating-garden-container")

		gardenContainer, err = worker.helper.createGardenContainer(containerSpec, fetchedImage, creatingContainer.Handle(), bindMounts)
		if err != nil {
			_, failedErr := creatingContainer.Failed()
			if failedErr != nil {
				logger.Error("failed-to-mark-container-as-failed", err)
			}
			metric.FailedContainers.Inc()

			logger.Error("failed-to-create-container-in-garden", err)
			return nil, err
		}

	}

	logger.Debug("created-container-in-garden")

	metric.ContainersCreated.Inc()
	createdContainer, err = creatingContainer.Created()
	if err != nil {
		logger.Error("failed-to-mark-container-as-created", err)

		_ = worker.gardenClient.Destroy(containerHandle)

		return nil, err
	}

	logger.Debug("created-container-in-db")

	return worker.helper.constructGardenWorkerContainer(
		logger,
		createdContainer,
		gardenContainer,
	)
}

func (worker *gardenWorker) getBindMounts(volumeMounts []VolumeMount, bindMountSources []BindMountSource) ([]garden.BindMount, error) {
	bindMounts := []garden.BindMount{}

	for _, mount := range bindMountSources {
		bindMount, found, mountErr := mount.VolumeOn(worker)
		if mountErr != nil {
			return nil, mountErr
		}
		if found {
			bindMounts = append(bindMounts, bindMount)
		}
	}

	for _, mount := range volumeMounts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: mount.Volume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})
	}
	return bindMounts, nil
}

func (worker *gardenWorker) fetchImageForContainer(
	ctx context.Context,
	logger lager.Logger,
	spec ImageSpec,
	teamID int,
	delegate ImageFetchingDelegate,
	resourceTypes atc.VersionedResourceTypes,
	creatingContainer db.CreatingContainer,
) (FetchedImage, error) {
	image, err := worker.imageFactory.GetImage(
		logger,
		worker,
		worker.volumeClient,
		spec,
		teamID,
		delegate,
		resourceTypes,
	)
	if err != nil {
		return FetchedImage{}, err
	}

	logger.Debug("fetching-image")
	return image.FetchForContainer(ctx, logger, creatingContainer)
}

type mountableLocalInput struct {
	desiredCOWParent Volume
	desiredMountPath string
}

type mountableRemoteInput struct {
	desiredArtifact  ArtifactSource
	desiredMountPath string
}

// creates volumes required to run any step:
// * scratch
// * working dir
// * input
// * output
func (worker *gardenWorker) createVolumes(
	ctx context.Context,
	logger lager.Logger,
	isPrivileged bool,
	creatingContainer db.CreatingContainer,
	spec ContainerSpec,
) ([]VolumeMount, error) {
	var volumeMounts []VolumeMount
	var ioVolumeMounts []VolumeMount

	scratchVolume, err := worker.volumeClient.FindOrCreateVolumeForContainer(
		logger,
		VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: isPrivileged,
		},
		creatingContainer,
		spec.TeamID,
		"/scratch",
	)
	if err != nil {
		return nil, err
	}

	scratchMount := VolumeMount{
		Volume:    scratchVolume,
		MountPath: "/scratch",
	}

	volumeMounts = append(volumeMounts, scratchMount)

	hasSpecDirInInputs := anyMountTo(spec.Dir, getDestinationPathsFromInputs(spec.Inputs))
	hasSpecDirInOutputs := anyMountTo(spec.Dir, getDestinationPathsFromOutputs(spec.Outputs))

	if spec.Dir != "" && !hasSpecDirInOutputs && !hasSpecDirInInputs {
		workdirVolume, volumeErr := worker.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: isPrivileged,
			},
			creatingContainer,
			spec.TeamID,
			spec.Dir,
		)
		if volumeErr != nil {
			return nil, volumeErr
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    workdirVolume,
			MountPath: spec.Dir,
		})
	}

	inputDestinationPaths := make(map[string]bool)

	localInputs := make([]mountableLocalInput, 0)
	nonlocalInputs := make([]mountableRemoteInput, 0)

	for _, inputSource := range spec.Inputs {
		inputSourceVolume, found, err := inputSource.Source().ExistsOn(logger, worker)
		if err != nil {
			return nil, err
		}
		cleanedInputPath := filepath.Clean(inputSource.DestinationPath())
		inputDestinationPaths[cleanedInputPath] = true

		if found {
			localInputs = append(localInputs, mountableLocalInput{
				desiredCOWParent: inputSourceVolume,
				desiredMountPath: cleanedInputPath,
			})
		} else {
			nonlocalInputs = append(nonlocalInputs, mountableRemoteInput{
				desiredArtifact:  inputSource.Source(),
				desiredMountPath: cleanedInputPath,
			})
		}
	}

	// we create COW volumes for task caches too, in case multiple builds
	// are running the same task
	cowMounts, err := worker.cloneLocalVolumes(
		logger,
		spec.TeamID,
		isPrivileged,
		creatingContainer,
		localInputs,
	)
	if err != nil {
		return nil, err
	}

	streamedMounts, err := worker.cloneRemoteVolumes(
		ctx,
		logger,
		spec.TeamID,
		isPrivileged,
		creatingContainer,
		nonlocalInputs,
	)
	if err != nil {
		return nil, err
	}

	ioVolumeMounts = append(ioVolumeMounts, cowMounts...)
	ioVolumeMounts = append(ioVolumeMounts, streamedMounts...)

	for _, outputPath := range spec.Outputs {
		cleanedOutputPath := filepath.Clean(outputPath)

		// reuse volume if output path is the same as input
		if inputDestinationPaths[cleanedOutputPath] {
			continue
		}

		outVolume, volumeErr := worker.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: isPrivileged,
			},
			creatingContainer,
			spec.TeamID,
			cleanedOutputPath,
		)
		if volumeErr != nil {
			return nil, volumeErr
		}

		ioVolumeMounts = append(ioVolumeMounts, VolumeMount{
			Volume:    outVolume,
			MountPath: cleanedOutputPath,
		})
	}

	sort.Sort(byMountPath(ioVolumeMounts))

	volumeMounts = append(volumeMounts, ioVolumeMounts...)
	return volumeMounts, nil
}

func (worker *gardenWorker) cloneLocalVolumes(
	logger lager.Logger,
	teamID int,
	privileged bool,
	container db.CreatingContainer,
	locals []mountableLocalInput,
) ([]VolumeMount, error) {
	mounts := make([]VolumeMount, len(locals))

	for i, localInput := range locals {
		inputVolume, err := worker.volumeClient.FindOrCreateCOWVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   localInput.desiredCOWParent.COWStrategy(),
				Privileged: privileged,
			},
			container,
			localInput.desiredCOWParent,
			teamID,
			localInput.desiredMountPath,
		)
		if err != nil {
			return nil, err
		}

		mounts[i] = VolumeMount{
			Volume:    inputVolume,
			MountPath: localInput.desiredMountPath,
		}
	}

	return mounts, nil
}

func (worker *gardenWorker) cloneRemoteVolumes(
	ctx context.Context,
	logger lager.Logger,
	teamID int,
	privileged bool,
	container db.CreatingContainer,
	nonLocals []mountableRemoteInput,
) ([]VolumeMount, error) {

	mounts := make([]VolumeMount, len(nonLocals))
	if len(nonLocals) <= 0 {
		return mounts, nil
	}

	ctx, span := tracing.StartSpan(ctx, "worker.cloneRemoteVolumes", tracing.Attrs{"container_id": container.Handle()})
	defer span.End()

	g, groupCtx := errgroup.WithContext(ctx)

	for i, nonLocalInput := range nonLocals {
		// this is to ensure each go func gets its own non changing copy of the iterator
		i, nonLocalInput := i, nonLocalInput
		inputVolume, err := worker.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			container,
			teamID,
			nonLocalInput.desiredMountPath,
		)
		if err != nil {
			return []VolumeMount{}, err
		}

		g.Go(func() error {
			if streamable, ok := nonLocalInput.desiredArtifact.(StreamableArtifactSource); ok {
				err = streamable.StreamTo(groupCtx, inputVolume)
				if err != nil {
					return err
				}
			}

			mounts[i] = VolumeMount{
				Volume:    inputVolume,
				MountPath: nonLocalInput.desiredMountPath,
			}

			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	logger.Debug("streamed-non-local-volumes", lager.Data{"volumes-streamed": len(nonLocals)})

	return mounts, nil
}

func (worker *gardenWorker) FindContainerByHandle(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Info("container-not-found")
			return nil, false, nil
		}

		logger.Error("failed-to-lookup-on-garden", err)
		return nil, false, err
	}

	createdContainer, found, err := worker.helper.dbTeamFactory.GetByID(teamID).FindCreatedContainerByHandle(handle)
	if err != nil {
		logger.Error("failed-to-lookup-in-db", err)
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	container, err := worker.helper.constructGardenWorkerContainer(logger, createdContainer, gardenContainer)
	if err != nil {
		logger.Error("failed-to-construct-container", err)
		return nil, false, err
	}

	return container, true, nil
}

func (worker *gardenWorker) Name() string {
	return worker.dbWorker.Name()
}

func (worker *gardenWorker) ResourceTypes() []atc.WorkerResourceType {
	return worker.dbWorker.ResourceTypes()
}

func (worker *gardenWorker) Tags() atc.Tags {
	return worker.dbWorker.Tags()
}

func (worker *gardenWorker) Ephemeral() bool {
	return worker.dbWorker.Ephemeral()
}

func (worker *gardenWorker) BuildContainers() int {
	return worker.buildContainers
}

func (worker *gardenWorker) Satisfies(logger lager.Logger, spec WorkerSpec) bool {
	workerTeamID := worker.dbWorker.TeamID()
	workerResourceTypes := worker.dbWorker.ResourceTypes()

	if spec.TeamID != workerTeamID && workerTeamID != 0 {
		return false
	}

	if spec.ResourceType != "" {
		underlyingType := determineUnderlyingTypeName(spec.ResourceType, spec.ResourceTypes)

		matchedType := false
		for _, t := range workerResourceTypes {
			if t.Type == underlyingType {
				matchedType = true
				break
			}
		}

		if !matchedType {
			return false
		}
	}

	if spec.Platform != "" {
		if spec.Platform != worker.dbWorker.Platform() {
			return false
		}
	}

	if !worker.tagsMatch(spec.Tags) {
		return false
	}

	return true
}

func determineUnderlyingTypeName(typeName string, resourceTypes atc.VersionedResourceTypes) string {
	resourceTypesMap := make(map[string]atc.VersionedResourceType)
	for _, resourceType := range resourceTypes {
		resourceTypesMap[resourceType.Name] = resourceType
	}
	underlyingTypeName := typeName
	underlyingType, ok := resourceTypesMap[underlyingTypeName]
	for ok {
		underlyingTypeName = underlyingType.Type
		underlyingType, ok = resourceTypesMap[underlyingTypeName]
		delete(resourceTypesMap, underlyingTypeName)
	}
	return underlyingTypeName
}

func (worker *gardenWorker) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", worker.dbWorker.Platform()),
	}

	for _, tag := range worker.dbWorker.Tags() {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}

func (worker *gardenWorker) IsOwnedByTeam() bool {
	return worker.dbWorker.TeamID() != 0
}

func (worker *gardenWorker) Uptime() time.Duration {
	return time.Since(worker.dbWorker.StartTime())
}

func (worker *gardenWorker) tagsMatch(tags []string) bool {
	workerTags := worker.dbWorker.Tags()
	if len(workerTags) > 0 && len(tags) == 0 {
		return false
	}

insert_coin:
	for _, stag := range tags {
		for _, wtag := range workerTags {
			if stag == wtag {
				continue insert_coin
			}
		}

		return false
	}

	return true
}

func (worker *gardenWorker) ActiveTasks() (int, error) {
	return worker.dbWorker.ActiveTasks()
}
func (worker *gardenWorker) IncreaseActiveTasks() error {
	return worker.dbWorker.IncreaseActiveTasks()
}
func (worker *gardenWorker) DecreaseActiveTasks() error {
	return worker.dbWorker.DecreaseActiveTasks()
}
