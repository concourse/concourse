//go:build linux

package spec

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	worldReadWrite = os.FileMode(0666)

	fuseDevice = specs.LinuxDevice{
		Path:     "/dev/fuse",
		Type:     "c",
		Major:    10,
		Minor:    229,
		FileMode: &worldReadWrite,
	}

	// Used by rootless podman/docker networking (pasta/slirp4netns); runc bind-mounts it from the host.
	tunDevice = specs.LinuxDevice{
		Path:     "/dev/net/tun",
		Type:     "c",
		Major:    10,
		Minor:    200,
		FileMode: &worldReadWrite,
	}

	// runc adds a list of devices by default.
	// The rule below gets appended to that list.
	// The rules along with some context can be found here:
	// https://github.com/opencontainers/runc/blob/94ae7afa36cc3b8f551e0bc67cf83e5efdf2555f/libcontainer/specconv/spec_linux.go#L50-L192
	// Currently these rules are highly permissive. We may want to re-visit them, but presently we don't know if they can
	// be overriden.
	// Linux docs about how cgroup device rules work:
	// https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/cgroup-v1/devices.rst
	DefaultContainerDevices = []specs.LinuxDeviceCgroup{
		// This allows use of the FUSE filesystem
		{Access: "rwm", Type: fuseDevice.Type, Major: new(fuseDevice.Major), Minor: new(fuseDevice.Minor), Allow: true}, // /dev/fuse
	}

	tunDeviceRule = specs.LinuxDeviceCgroup{Access: "rwm", Type: tunDevice.Type, Major: new(tunDevice.Major), Minor: new(tunDevice.Minor), Allow: true}
)

func Devices(privilegedMode PrivilegedMode, privileged bool) []specs.LinuxDevice {
	if !privileged || privilegedMode == IgnorePrivilegedMode {
		return nil
	}
	devices := []specs.LinuxDevice{fuseDevice}
	if privilegedMode == RootlessPrivilegedMode {
		devices = append(devices, tunDevice)
	}
	return devices
}

// ParseAllowedDevices parses a list of device rule strings into LinuxDeviceCgroup entries.
func ParseAllowedDevices(rules []string) ([]specs.LinuxDeviceCgroup, error) {
	devices := make([]specs.LinuxDeviceCgroup, 0, len(rules))
	for _, rule := range rules {
		d, err := ParseDeviceRule(rule)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// ParseDeviceRule parses a device rule string in the format "type major:minor access".
func ParseDeviceRule(s string) (specs.LinuxDeviceCgroup, error) {
	parts := strings.Fields(s)
	if len(parts) != 3 {
		return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid device rule %q: expected format 'type major:minor access'", s)
	}

	devType := parts[0]
	switch devType {
	case "a", "b", "c":
	default:
		return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid device type %q: must be a, b, or c", devType)
	}

	majorMinor := strings.SplitN(parts[1], ":", 2)
	if len(majorMinor) != 2 {
		return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid major:minor %q: expected format 'major:minor'", parts[1])
	}

	var major, minor *int64
	if majorMinor[0] != "*" {
		v, err := strconv.ParseInt(majorMinor[0], 10, 64)
		if err != nil {
			return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid major number %q: %w", majorMinor[0], err)
		}
		major = &v
	}
	if majorMinor[1] != "*" {
		v, err := strconv.ParseInt(majorMinor[1], 10, 64)
		if err != nil {
			return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid minor number %q: %w", majorMinor[1], err)
		}
		minor = &v
	}

	access := parts[2]
	for _, c := range access {
		switch c {
		case 'r', 'w', 'm':
		default:
			return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid access character %q in %q: must be r, w, or m", string(c), access)
		}
	}

	return specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   devType,
		Major:  major,
		Minor:  minor,
		Access: access,
	}, nil
}
