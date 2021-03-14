package gardenruntime

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

var ErrResourceConfigCheckSessionExpired = errors.New("no db container was found for owner")
var ErrMissingVolume = errors.New("volume mounted to container is missing")
var ErrImageArtifactVolumeNotFound = errors.New("image artifact volume was not found")
var ErrEmptyImageSpec = errors.New("image spec is empty")
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

type InputNotFoundError struct {
	Input runtime.Input
}

func (e InputNotFoundError) Error() string {
	return fmt.Sprintf("input '%s' (volume '%s') not found", e.Input.DestinationPath, e.Input.VolumeHandle)
}
