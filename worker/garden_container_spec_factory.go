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
}

func NewGardenContainerSpecFactory(logger lager.Logger, baggageclaimClient baggageclaim.Client, imageFetcher ImageFetcher) gardenContainerSpecFactory {
	return gardenContainerSpecFactory{
		logger:             logger,
		baggageclaimClient: baggageclaimClient,
		imageFetcher:       imageFetcher,
		volumeMounts:       map[string]string{},
		volumeHandles:      nil,
		releaseAfterCreate: []releasable{},
	}
}

func (factory *gardenContainerSpecFactory) BuildContainerSpec(
	spec ContainerSpec,
	resourceTypes []atc.WorkerResourceType,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	workerClient Client,
	customTypes atc.ResourceTypes,
) (garden.ContainerSpec, error) {
	var err error

	resourceTypeContainerSpec, ok := spec.(ResourceTypeContainerSpec)
	if ok {
		customType, found := lookupCustomType(resourceTypeContainerSpec.Type, customTypes)
		if found {
			customTypes = customTypes.Without(resourceTypeContainerSpec.Type)

			resourceTypeContainerSpec.ImageResourcePointer = &atc.TaskImageConfig{
				Source: customType.Source,
				Type:   customType.Type,
			}

			spec = resourceTypeContainerSpec
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

	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec, err = factory.BuildResourceContainerSpec(s, gardenSpec, resourceTypes)
	case TaskContainerSpec:
		gardenSpec, err = factory.BuildTaskContainerSpec(s, gardenSpec, cancel, delegate, id, metadata, workerClient)
	default:
		return garden.ContainerSpec{}, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}
	if err != nil {
		return garden.ContainerSpec{}, err
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

	if factory.user != "" {
		gardenSpec.Properties["user"] = factory.user
	}
	return gardenSpec, nil
}

func (factory *gardenContainerSpecFactory) BuildResourceContainerSpec(
	spec ResourceTypeContainerSpec,
	gardenSpec garden.ContainerSpec,
	resourceTypes []atc.WorkerResourceType,
) (garden.ContainerSpec, error) {
	if len(spec.Mounts) > 0 && spec.Cache.Volume != nil {
		return gardenSpec, errors.New("a container may not have mounts and a cache")
	}

	gardenSpec.Privileged = true
	gardenSpec.Env = append(gardenSpec.Env, spec.Env...)

	if spec.Ephemeral {
		gardenSpec.Properties[ephemeralPropertyName] = "true"
	}

	if spec.Cache.Volume != nil && spec.Cache.MountPath != "" {
		gardenSpec.BindMounts = []garden.BindMount{
			{
				SrcPath: spec.Cache.Volume.Path(),
				DstPath: spec.Cache.MountPath,
				Mode:    garden.BindMountModeRW,
			},
		}

		factory.volumeHandles = append(factory.volumeHandles, spec.Cache.Volume.Handle())
		factory.volumeMounts[spec.Cache.Volume.Handle()] = spec.Cache.MountPath
	}

	var err error
	gardenSpec, err = factory.createVolumes(gardenSpec, spec.Mounts)
	if err != nil {
		return gardenSpec, err
	}

	if spec.ImageResourcePointer == nil {
		for _, t := range resourceTypes {
			if t.Type == spec.Type {
				gardenSpec.RootFSPath = t.Image
				return gardenSpec, nil
			}
		}

		return gardenSpec, ErrUnsupportedResourceType
	}

	return gardenSpec, nil
}

func lookupCustomType(typeName string, customTypes atc.ResourceTypes) (atc.ResourceType, bool) {
	for _, customType := range customTypes {
		if customType.Name == typeName {
			return customType, true
		}
	}
	return atc.ResourceType{}, false
}

func (factory *gardenContainerSpecFactory) BuildTaskContainerSpec(
	spec TaskContainerSpec,
	gardenSpec garden.ContainerSpec,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	workerClient Client,
) (garden.ContainerSpec, error) {
	if spec.ImageResourcePointer == nil {
		gardenSpec.RootFSPath = spec.Image
	}

	gardenSpec.Privileged = spec.Privileged

	var err error
	gardenSpec, err = factory.createVolumes(gardenSpec, spec.Inputs)
	if err != nil {
		return gardenSpec, err
	}

	for _, mount := range spec.Outputs {
		volume := mount.Volume
		gardenSpec.BindMounts = append(gardenSpec.BindMounts, garden.BindMount{
			SrcPath: volume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})

		factory.volumeHandles = append(factory.volumeHandles, volume.Handle())
		factory.volumeMounts[volume.Handle()] = mount.MountPath
	}

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

		factory.releaseAfterCreate = append(factory.releaseAfterCreate, cowVolume)

		containerSpec.BindMounts = append(containerSpec.BindMounts, garden.BindMount{
			SrcPath: cowVolume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})

		factory.volumeHandles = append(factory.volumeHandles, cowVolume.Handle())
		factory.volumeMounts[cowVolume.Handle()] = mount.MountPath
	}

	return containerSpec, nil
}
