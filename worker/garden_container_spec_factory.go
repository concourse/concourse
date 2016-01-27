package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

const ephemeralPropertyName = "concourse:ephemeral"
const volumePropertyName = "concourse:volumes"
const volumeMountsPropertyName = "concourse:volume-mounts"

type gardenContainerSpecFactory struct {
	logger             lager.Logger
	baggageclaimClient baggageclaim.Client
	volumeMounts       map[string]string
	volumeHandles      []string
	cowVolumes         []Volume
}

func NewGardenContainerSpecFactory(logger lager.Logger, baggageclaimClient baggageclaim.Client) gardenContainerSpecFactory {
	return gardenContainerSpecFactory{
		logger:             logger,
		baggageclaimClient: baggageclaimClient,
		volumeMounts:       map[string]string{},
		volumeHandles:      nil,
		cowVolumes:         []Volume{},
	}
}

func (factory *gardenContainerSpecFactory) BuildContainerSpec(spec ContainerSpec, resourceTypes []atc.WorkerResourceType) (garden.ContainerSpec, error) {
	var err error
	gardenSpec := garden.ContainerSpec{
		Properties: garden.Properties{},
	}

	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec, err = factory.BuildResourceContainerSpec(s, gardenSpec, resourceTypes)
	case TaskContainerSpec:
		gardenSpec, err = factory.BuildTaskContainerSpec(s, gardenSpec)
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
	return gardenSpec, nil
}

func (factory *gardenContainerSpecFactory) BuildResourceContainerSpec(spec ResourceTypeContainerSpec, gardenSpec garden.ContainerSpec, resourceTypes []atc.WorkerResourceType) (garden.ContainerSpec, error) {
	if len(spec.Mounts) > 0 && spec.Cache.Volume != nil {
		return gardenSpec, errors.New("a container may not have mounts and a cache")
	}

	gardenSpec.Privileged = true
	gardenSpec.Env = spec.Env

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

	for _, t := range resourceTypes {
		if t.Type == spec.Type {
			gardenSpec.RootFSPath = t.Image
			return gardenSpec, nil
		}
	}

	return gardenSpec, ErrUnsupportedResourceType
}

func (factory *gardenContainerSpecFactory) BuildTaskContainerSpec(spec TaskContainerSpec, gardenSpec garden.ContainerSpec) (garden.ContainerSpec, error) {
	if spec.ImageVolume != nil {
		gardenSpec.RootFSPath = path.Join(spec.ImageVolume.Path(), "rootfs")
		factory.volumeHandles = append(factory.volumeHandles, spec.ImageVolume.Handle())
	} else {
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
	for _, cow := range factory.cowVolumes {
		cow.Release(0)
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

		// release *after* container creation
		factory.cowVolumes = append(factory.cowVolumes, cowVolume)

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
