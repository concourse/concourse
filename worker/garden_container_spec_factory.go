package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

const ephemeralPropertyName = "concourse:ephemeral"
const volumePropertyName = "concourse:volumes"
const volumeMountsPropertyName = "concourse:volume-mounts"
const userPropertyName = "user"

type releasable interface {
	Release(*time.Duration)
}

type gardenContainerSpecFactory struct {
	logger             lager.Logger
	baggageclaimClient baggageclaim.Client
	imageFetcher       ImageFetcher
	releaseAfterCreate []releasable
	db                 GardenWorkerDB
}

func NewGardenContainerSpecFactory(logger lager.Logger, baggageclaimClient baggageclaim.Client, imageFetcher ImageFetcher, db GardenWorkerDB) gardenContainerSpecFactory {
	return gardenContainerSpecFactory{
		logger:             logger,
		baggageclaimClient: baggageclaimClient,
		imageFetcher:       imageFetcher,
		releaseAfterCreate: []releasable{},
		db:                 db,
	}
}

func (factory *gardenContainerSpecFactory) BuildContainerSpec(
	spec ContainerSpec,
	resourceTypes []atc.WorkerResourceType,
	workerTags atc.Tags,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	workerClient Client,
	customTypes atc.ResourceTypes,
) (garden.ContainerSpec, error) {
	var (
		volumeHandles []string
		volumeMounts  []VolumeMount
		gardenSpec    garden.ContainerSpec
	)
	volumeMountPaths := map[baggageclaim.Volume]string{}

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		for _, customType := range customTypes {
			if customType.Name == s.Type {
				customTypes = customTypes.Without(s.Type)
				s.ImageResourcePointer = &atc.TaskImageConfig{
					Source: customType.Source,
					Type:   customType.Type,
				}
			}
		}

		if len(s.Mounts) > 0 && s.Cache.Volume != nil {
			return garden.ContainerSpec{}, errors.New("a container may not have mounts and a cache")
		}

		volumeMounts = s.Mounts

		if s.Cache.Volume != nil && s.Cache.MountPath != "" {
			volumeHandles = append(volumeHandles, s.Cache.Volume.Handle())
			volumeMountPaths[s.Cache.Volume] = s.Cache.MountPath
		}

		baseGardenSpec, imageFetched, image, err := factory.baseGardenSpec(
			s.ImageResourcePointer,
			workerTags,
			cancel,
			delegate,
			id,
			metadata,
			workerClient,
			customTypes,
		)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec = baseGardenSpec

		if imageFetched {
			imageVolume := image.Volume()
			volumeHandles = append(volumeHandles, imageVolume.Handle())
			factory.releaseAfterCreate = append(factory.releaseAfterCreate, image)
			gardenSpec.Properties[userPropertyName] = image.Metadata().User
		} else {
			gardenSpec.Properties[userPropertyName] = ""
		}

		gardenSpec.Privileged = true
		gardenSpec.Env = append(gardenSpec.Env, s.Env...)

		if s.Ephemeral {
			gardenSpec.Properties[ephemeralPropertyName] = "true"
		}

		if s.ImageResourcePointer == nil {
			for _, t := range resourceTypes {
				if t.Type == s.Type {
					gardenSpec.RootFSPath = t.Image
					break dance
				}
			}

			return garden.ContainerSpec{}, ErrUnsupportedResourceType
		}
	case TaskContainerSpec:
		volumeMounts = s.Inputs

		for _, mount := range s.Outputs {
			volume := mount.Volume
			volumeHandles = append(volumeHandles, volume.Handle())
			volumeMountPaths[volume] = mount.MountPath
		}

		baseGardenSpec, imageFetched, image, err := factory.baseGardenSpec(
			s.ImageResourcePointer,
			workerTags,
			cancel,
			delegate,
			id,
			metadata,
			workerClient,
			customTypes,
		)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec = baseGardenSpec

		if imageFetched {
			imageVolume := image.Volume()
			volumeHandles = append(volumeHandles, imageVolume.Handle())
			factory.releaseAfterCreate = append(factory.releaseAfterCreate, image)
			gardenSpec.Properties[userPropertyName] = image.Metadata().User
		} else {
			gardenSpec.Properties[userPropertyName] = ""
		}

		gardenSpec.Privileged = s.Privileged

		if s.ImageResourcePointer == nil {
			gardenSpec.RootFSPath = s.Image
		}
	default:
		return garden.ContainerSpec{}, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	newVolumeHandles, newVolumeMountPaths, err := factory.createVolumes(gardenSpec, volumeMounts)
	if err != nil {
		return garden.ContainerSpec{}, err
	}

	for _, h := range newVolumeHandles {
		volumeHandles = append(volumeHandles, h)
	}

	for volume, mountPath := range newVolumeMountPaths {
		volumeMountPaths[volume] = mountPath
	}

	for volume, mount := range volumeMountPaths {
		gardenSpec.BindMounts = append(gardenSpec.BindMounts, garden.BindMount{
			SrcPath: volume.Path(),
			DstPath: mount,
			Mode:    garden.BindMountModeRW,
		})
	}

	if len(volumeHandles) > 0 {
		volumesJSON, err := json.Marshal(volumeHandles)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec.Properties[volumePropertyName] = string(volumesJSON)

		volumeHandleMounts := map[string]string{}

		for k, v := range volumeMountPaths {
			volumeHandleMounts[k.Handle()] = v
		}

		mountsJSON, err := json.Marshal(volumeHandleMounts)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec.Properties[volumeMountsPropertyName] = string(mountsJSON)
	}

	return gardenSpec, nil
}

func (factory *gardenContainerSpecFactory) ReleaseVolumes() {
	for _, releasable := range factory.releaseAfterCreate {
		releasable.Release(nil)
	}
}

func (factory *gardenContainerSpecFactory) baseGardenSpec(
	taskImageConfig *atc.TaskImageConfig,
	workerTags atc.Tags,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	workerClient Client,
	customTypes atc.ResourceTypes,
) (garden.ContainerSpec, bool, Image, error) {
	if taskImageConfig != nil {
		image, err := factory.imageFetcher.FetchImage(
			factory.logger,
			*taskImageConfig,
			cancel,
			id,
			metadata,
			delegate,
			workerClient,
			workerTags,
			customTypes,
		)
		if err != nil {
			return garden.ContainerSpec{}, false, nil, err
		}

		gardenSpec := garden.ContainerSpec{
			Properties: garden.Properties{},
			RootFSPath: path.Join(image.Volume().Path(), "rootfs"),
			Env:        image.Metadata().Env,
		}

		return gardenSpec, true, image, nil
	}

	gardenSpec := garden.ContainerSpec{
		Properties: garden.Properties{},
	}
	return gardenSpec, false, nil, nil
}

func (factory *gardenContainerSpecFactory) createVolumes(containerSpec garden.ContainerSpec, mounts []VolumeMount) ([]string, map[baggageclaim.Volume]string, error) {
	var volumeHandles []string
	volumeMountPaths := map[baggageclaim.Volume]string{}

	for _, mount := range mounts {
		cowVolume, err := factory.baggageclaimClient.CreateVolume(factory.logger, baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: mount.Volume,
			},
			Privileged: containerSpec.Privileged,
			TTL:        VolumeTTL,
		})
		if err != nil {
			return []string{}, map[baggageclaim.Volume]string{}, err
		}

		factory.releaseAfterCreate = append(factory.releaseAfterCreate, cowVolume)

		err = factory.db.InsertCOWVolume(mount.Volume.Handle(), cowVolume.Handle(), VolumeTTL)
		if err != nil {
			return []string{}, map[baggageclaim.Volume]string{}, err
		}

		volumeHandles = append(volumeHandles, cowVolume.Handle())
		volumeMountPaths[cowVolume] = mount.MountPath

		factory.logger.Info("created-cow-volume", lager.Data{
			"original-volume-handle": mount.Volume.Handle(),
			"cow-volume-handle":      cowVolume.Handle(),
		})
	}

	return volumeHandles, volumeMountPaths, nil
}
