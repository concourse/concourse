package runtimetest

import (
	"context"
	"io"

	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
)

type Volume struct {
	VolumeHandle string
}

func NewVolume(handle string) Volume {
	return Volume{
		VolumeHandle: handle,
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

func (v Volume) DBVolume() db.CreatedVolume {
	panic("unimplemented")
}
