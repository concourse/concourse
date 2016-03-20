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

type releasable interface {
	Release(*time.Duration)
}

type gardenContainerSpecFactory struct {
	logger             lager.Logger
	baggageclaimClient baggageclaim.Client
	imageFetcher       ImageFetcher
	volumeMounts       map[string]string
	volumeHandles      []string
	user               string
	releaseAfterCreate []releasable
	db                 GardenWorkerDB
}

func NewGardenContainerSpecFactory(logger lager.Logger, baggageclaimClient baggageclaim.Client, imageFetcher ImageFetcher, db GardenWorkerDB) gardenContainerSpecFactory {
	return gardenContainerSpecFactory{
		logger:             logger,
		baggageclaimClient: baggageclaimClient,
		imageFetcher:       imageFetcher,
		volumeMounts:       map[string]string{},
		volumeHandles:      nil,
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
	resourceTypeContainerSpec, ok := spec.(ResourceTypeContainerSpec)
	if ok {
		for _, customType := range customTypes {
			if customType.Name == resourceTypeContainerSpec.Type {
				customTypes = customTypes.Without(resourceTypeContainerSpec.Type)

				resourceTypeContainerSpec.ImageResourcePointer = &atc.TaskImageConfig{
					Source: customType.Source,
					Type:   customType.Type,
				}

				spec = resourceTypeContainerSpec
			}
		}
	}

	imageResourceConfig, hasImageResource := spec.ImageResource()
	var gardenSpec garden.ContainerSpec
	if hasImageResource {
		image, err := factory.imageFetcher.FetchImage(
			factory.logger,
			imageResourceConfig,
			cancel,
			id,
			metadata,
			delegate,
			workerClient,
			workerTags,
			customTypes,
		)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		imageVolume := image.Volume()

		factory.volumeHandles = append(factory.volumeHandles, imageVolume.Handle())
		factory.releaseAfterCreate = append(factory.releaseAfterCreate, image)
		factory.user = image.Metadata().User

		gardenSpec = garden.ContainerSpec{
			Properties: garden.Properties{},
			RootFSPath: path.Join(imageVolume.Path(), "rootfs"),
			Env:        image.Metadata().Env,
		}
	} else {
		gardenSpec = garden.ContainerSpec{
			Properties: garden.Properties{},
		}
	}

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		if len(s.Mounts) > 0 && s.Cache.Volume != nil {
			return gardenSpec, errors.New("a container may not have mounts and a cache")
		}

		gardenSpec.Privileged = true
		gardenSpec.Env = append(gardenSpec.Env, s.Env...)

		if s.Ephemeral {
			gardenSpec.Properties[ephemeralPropertyName] = "true"
		}

		if s.Cache.Volume != nil && s.Cache.MountPath != "" {
			gardenSpec.BindMounts = []garden.BindMount{
				{
					SrcPath: s.Cache.Volume.Path(),
					DstPath: s.Cache.MountPath,
					Mode:    garden.BindMountModeRW,
				},
			}

			factory.volumeHandles = append(factory.volumeHandles, s.Cache.Volume.Handle())
			factory.volumeMounts[s.Cache.Volume.Handle()] = s.Cache.MountPath
		}

		var err error
		gardenSpec, err = factory.createVolumes(gardenSpec, s.Mounts)
		if err != nil {
			return garden.ContainerSpec{}, err
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

		break dance
	case TaskContainerSpec:
		if s.ImageResourcePointer == nil {
			gardenSpec.RootFSPath = s.Image
		}

		gardenSpec.Privileged = s.Privileged

		var err error
		gardenSpec, err = factory.createVolumes(gardenSpec, s.Inputs)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		for _, mount := range s.Outputs {
			volume := mount.Volume
			gardenSpec.BindMounts = append(gardenSpec.BindMounts, garden.BindMount{
				SrcPath: volume.Path(),
				DstPath: mount.MountPath,
				Mode:    garden.BindMountModeRW,
			})

			factory.volumeHandles = append(factory.volumeHandles, volume.Handle())
			factory.volumeMounts[volume.Handle()] = mount.MountPath
		}

		break dance
	default:
		return garden.ContainerSpec{}, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	if len(factory.volumeHandles) > 0 {
		volumesJSON, err := json.Marshal(factory.volumeHandles)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec.Properties[volumePropertyName] = string(volumesJSON)

		mountsJSON, err := json.Marshal(factory.volumeMounts)
		if err != nil {
			return garden.ContainerSpec{}, err
		}

		gardenSpec.Properties[volumeMountsPropertyName] = string(mountsJSON)
	}

	gardenSpec.Properties["user"] = factory.user

	return gardenSpec, nil
}

func (factory *gardenContainerSpecFactory) ReleaseVolumes() {
	for _, cow := range factory.releaseAfterCreate {
		cow.Release(nil)
	}
}

func (factory *gardenContainerSpecFactory) createVolumes(containerSpec garden.ContainerSpec, mounts []VolumeMount) (garden.ContainerSpec, error) {
	for _, mount := range mounts {
		cowVolume, err := factory.baggageclaimClient.CreateVolume(factory.logger, baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: mount.Volume,
			},
			Privileged: containerSpec.Privileged,
			TTL:        VolumeTTL,
		})
		if err != nil {
			return containerSpec, err
		}

		err = factory.db.InsertCOWVolume(mount.Volume.Handle(), cowVolume.Handle(), VolumeTTL)
		if err != nil {
			return containerSpec, err
		}

		factory.releaseAfterCreate = append(factory.releaseAfterCreate, cowVolume)

		containerSpec.BindMounts = append(containerSpec.BindMounts, garden.BindMount{
			SrcPath: cowVolume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})

		factory.volumeHandles = append(factory.volumeHandles, cowVolume.Handle())
		factory.volumeMounts[cowVolume.Handle()] = mount.MountPath

		factory.logger.Info("created-cow-volume", lager.Data{
			"original-volume-handle": mount.Volume.Handle(),
			"cow-volume-handle":      cowVolume.Handle(),
		})
	}

	return containerSpec, nil
}
