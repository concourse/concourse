package spec

import "github.com/opencontainers/runtime-spec/specs-go"

var (
	// runc adds a list of devices by default.
	// The rule below gets appended to that list.
	// The rules along with some context can be found here:
	// https://github.com/opencontainers/runc/blob/94ae7afa36cc3b8f551e0bc67cf83e5efdf2555f/libcontainer/specconv/spec_linux.go#L50-L192
	// Currently these rules are highly permissive. We may want to re-visit them, but presently we don't know if they can
	// be overriden.
	// Linux docs about how cgroup device rules work:
	// https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/cgroup-v1/devices.rst
	AnyContainerDevices = []specs.LinuxDeviceCgroup{
		// This allows use of the FUSE filesystem. We are following Guardian in this case, and aren't sure of the
		// exact use cases.
		{Access: "rwm", Type: "c", Major: intRef(10), Minor: intRef(229), Allow: true}, 	// /dev/fuse
	}
)

func intRef(i int64) *int64  { return &i }