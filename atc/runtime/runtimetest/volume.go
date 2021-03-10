package runtimetest

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
)

type Volume struct {
	VolumeHandle             string
	ResourceCacheInitialized bool
	DBVolume_                *dbfakes.FakeCreatedVolume
}

func NewVolume(handle string) *Volume {
	dbVolume := new(dbfakes.FakeCreatedVolume)
	dbVolume.HandleReturns(handle)
	return &Volume{
		VolumeHandle: handle,
		DBVolume_:    dbVolume,
	}
}

func (v Volume) Handle() string {
	return v.VolumeHandle
}

func (v Volume) StreamIn(ctx context.Context, path string, compression compression.Compression, reader io.Reader) error {
	panic("unimplemented")
}

func (v Volume) StreamOut(ctx context.Context, path string, compression compression.Compression) (io.ReadCloser, error) {
	panic("unimplemented")
}

func (v *Volume) InitializeResourceCache(_ lager.Logger, _ db.UsedResourceCache) error {
	v.ResourceCacheInitialized = true
	return nil
}

func (v Volume) DBVolume() db.CreatedVolume {
	return v.DBVolume_
}
