package healthcheck

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type Worker struct {
	ContainerProvider ContainerProvider
	VolumeProvider    VolumeProvider
	TTL               time.Duration
}

func (w *Worker) Check(ctx context.Context) error {
	handle, err := createHandle()
	if err != nil {
		return errors.Wrapf(err,
			"couldn't create healthcheck handle")
	}

	rootfs, err := w.VolumeProvider.Create(ctx, handle, w.TTL)
	if err != nil {
		return errors.Wrapf(err,
			"failed to create volume")
	}

	err = w.ContainerProvider.Create(ctx, handle, rootfs.Path, w.TTL)
	if err != nil {
		return errors.Wrapf(err,
			"failed to create container")
	}

	return nil
}
