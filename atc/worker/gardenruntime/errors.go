package gardenruntime

import (
	"errors"
	"fmt"
)

var ErrMissingVolume = errors.New("volume mounted to container is missing")
var ErrBaseResourceTypeNotFound = errors.New("base resource type not found")
var ErrUnsupportedResourceType = errors.New("unsupported resource type")

type CreatedVolumeNotFoundError struct {
	Handle     string
	WorkerName string
}

func (e CreatedVolumeNotFoundError) Error() string {
	return fmt.Sprintf("volume '%s' disappeared from worker '%s'", e.Handle, e.WorkerName)
}

type MalformedMetadataError struct {
	UnmarshalError error
}

func (err MalformedMetadataError) Error() string {
	return fmt.Sprintf("malformed image metadata: %s", err.UnmarshalError)
}

type MountedVolumeMissingFromWorker struct {
	Handle     string
	WorkerName string
}

func (e MountedVolumeMissingFromWorker) Error() string {
	return fmt.Sprintf("volume mounted to container is missing '%s' from worker '%s'", e.Handle, e.WorkerName)
}
