package healthcheck

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type Worker struct {
	ContainerProvider ContainerProvider
	VolumeProvider    VolumeProvider
}

func (w *Worker) Check(ctx context.Context) error {
	handle, err := createHandle()
	if err != nil {
		return errors.Wrapf(err,
			"couldn't create healthcheck handle")
	}

	rootfs, err := w.VolumeProvider.Create(ctx, handle)
	if err != nil {
		return errors.Wrapf(err,
			"failed to create volume")
	}

	err = w.ContainerProvider.Create(ctx, handle, rootfs.Path)
	if err != nil {
		err = errors.Wrapf(err, "failed to create container")

		volDestructionErr := w.VolumeProvider.Destroy(ctx, handle)
		if volDestructionErr != nil {
			volDestructionErr = errors.Wrapf(volDestructionErr,
				"failed to destroy volume")
		}

		return multierror.Append(err, volDestructionErr)
	}

	err = w.ContainerProvider.Destroy(ctx, handle)
	if err != nil {
		return errors.Wrapf(err,
			"failed to destroy container")
	}

	err = w.VolumeProvider.Destroy(ctx, handle)
	if err != nil {
		return errors.Wrapf(err,
			"failed to destroy volume")
	}

	return nil
}
