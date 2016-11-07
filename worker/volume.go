package worker

import "github.com/concourse/baggageclaim"

//go:generate counterfeiter . VolumeFactoryDB

type VolumeFactoryDB interface {
	ReapVolume(handle string) error
}

//go:generate counterfeiter . Volume

type Volume interface {
	baggageclaim.Volume
}

type VolumeMount struct {
	Volume    Volume
	MountPath string
}
