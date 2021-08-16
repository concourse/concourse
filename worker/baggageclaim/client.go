package baggageclaim

import (
	"context"
	"encoding/json"
	"io"
)

type Encoding string

const GzipEncoding Encoding = "gzip"
const ZstdEncoding Encoding = "zstd"

//go:generate counterfeiter . Client

// Client represents a client connection to a BaggageClaim server.
type Client interface {
	// CreateVolume will create a volume on the remote server. By passing in a
	// VolumeSpec with a different strategy you can choose the type of volume
	// that you want to create.
	//
	// CreateVolume returns the volume that was created or an error as to why it
	// could not be created.
	CreateVolume(context.Context, string, VolumeSpec) (Volume, error)

	// ListVolumes lists the volumes that are present on the server. A
	// VolumeProperties object can be passed in to filter the volumes that are in
	// the response.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// ListVolumes returns the volumes that were found or an error as to why they
	// could not be listed.
	ListVolumes(context.Context, VolumeProperties) (Volumes, error)

	// LookupVolume finds a volume that is present on the server. It takes a
	// string that corresponds to the Handle of the Volume.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// LookupVolume returns a bool if the volume is found with the matching volume
	// or an error as to why the volume could not be found.
	LookupVolume(context.Context, string) (Volume, bool, error)

	// DestroyVolumes deletes the list of volumes that is present on the server. It takes
	// a string of volumes
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// DestroyVolumes returns an error if any of the volume deletion fails. It does not
	// return an error if volumes were not found on the server.
	// DestroyVolumes returns an error as to why one or more volumes could not be deleted.
	DestroyVolumes(context.Context, []string) error

	// DestroyVolume deletes the volume with the provided handle that is present on the server.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// DestroyVolume returns an error if the volume deletion fails. It does not
	// return an error if the volume was not found on the server.
	DestroyVolume(context.Context, string) error
}

//go:generate counterfeiter . Volume

// Volume represents a volume in the BaggageClaim system.
type Volume interface {
	// Handle returns a per-server unique identifier for the volume. The URL of
	// the server and a handle is enough to universally identify a volume.
	Handle() string

	// Path returns the filesystem path to the volume on the server. This can be
	// supplied to other systems in order to let them use the volume.
	Path() string

	// SetProperty sets a property on the Volume. Properties can be used to
	// filter the results in the ListVolumes call above.
	SetProperty(ctx context.Context, key string, value string) error

	// SetPrivileged namespaces or un-namespaces the UID/GID ownership of the
	// volume's contents.
	SetPrivileged(context.Context, bool) error

	// GetPrivileged returns a bool indicating if the volume is privileged.
	GetPrivileged(context.Context) (bool, error)

	// StreamIn calls BaggageClaim API endpoint in order to initialize tarStream
	// to stream the contents of the Reader into this volume at the specified path.
	StreamIn(ctx context.Context, path string, encoding Encoding, tarStream io.Reader) error

	StreamOut(ctx context.Context, path string, encoding Encoding) (io.ReadCloser, error)

	// Properties returns the currently set properties for a Volume. An error is
	// returned if these could not be retrieved.
	Properties(context.Context) (VolumeProperties, error)

	// Destroy removes the volume and its contents. Note that it does not
	// safeguard against child volumes being present.
	Destroy(context.Context) error

	// GetStreamInP2pUrl returns a modified StreamIn URL for this volume. The
	// returned URL contains a hostname that is reachable by other baggageclaim
	// servers on the same network. The URL can be passed to another
	// baggageclaim server to stream the contents of its source volume into
	// this target volume.
	GetStreamInP2pUrl(ctx context.Context, path string) (string, error)

	// StreamP2pOut streams the contents of this volume directly to another
	// baggageclaim server on the same network.
	StreamP2pOut(ctx context.Context, path string, streamInURL string, encoding Encoding) error
}

// Volumes represents a list of Volume object.
type Volumes []Volume

func (v Volumes) Handles() []string {
	var handles []string
	for _, vol := range v {
		handles = append(handles, vol.Handle())
	}
	return handles
}

// VolumeProperties represents the properties for a particular volume.
type VolumeProperties map[string]string

// VolumeSpec is a specification representing the kind of volume that you'd
// like from the server.
type VolumeSpec struct {
	// Strategy is the information that the server requires to materialise the
	// volume. There are examples of these in this package.
	Strategy Strategy

	// Properties is the set of initial properties that the Volume should have.
	Properties VolumeProperties

	// Privileged is used to determine whether or not we need to perform a UID
	// translation of the files in the volume so that they can be read by a
	// non-privileged user.
	Privileged bool
}

type Strategy interface {
	Encode() *json.RawMessage
}

// ImportStrategy creates a volume by copying a directory from the host.
type ImportStrategy struct {
	// The location on the host to import. If the path is a directory, its
	// contents will be copied in. If the path is a file, it is assumed to be a
	// .tar.gz file, and its contents will be unpacked in to the volume.
	Path string

	// Follow symlinks and import them as files instead of links.
	FollowSymlinks bool
}

func (strategy ImportStrategy) Encode() *json.RawMessage {
	payload, _ := json.Marshal(struct {
		Type           string `json:"type"`
		Path           string `json:"path"`
		FollowSymlinks bool   `json:"follow_symlinks"`
	}{
		Type:           "import",
		Path:           strategy.Path,
		FollowSymlinks: strategy.FollowSymlinks,
	})

	msg := json.RawMessage(payload)
	return &msg
}

// COWStrategy creates a Copy-On-Write layer of another Volume.
type COWStrategy struct {
	// The parent volume that we should base the new volume on.
	Parent Volume
}

func (strategy COWStrategy) Encode() *json.RawMessage {
	payload, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Volume string `json:"volume"`
	}{
		Type:   "cow",
		Volume: strategy.Parent.Handle(),
	})

	msg := json.RawMessage(payload)
	return &msg
}

// EmptyStrategy created a new empty volume.
type EmptyStrategy struct{}

func (EmptyStrategy) Encode() *json.RawMessage {
	msg := json.RawMessage(`{"type":"empty"}`)
	return &msg
}
