//go:build linux

package spec

import (
	"github.com/containers/common/pkg/config"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const DefaultInitBinPath = "/usr/local/concourse/bin/init"

var (
	DefaultContainerMounts = []specs.Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "ro"},
		},
		{
			Destination: "/sys/fs/cgroup",
			Type:        systemCgroupType(),
			Source:      systemCgroupType(),
			Options:     []string{"ro", "nosuid", "noexec", "nodev"},
		},
	}
)

func ContainerMounts(privilegedMode PrivilegedMode, privileged bool, initBinPath string) []specs.Mount {
	mounts := make([]specs.Mount, 0, len(DefaultContainerMounts)+1)
	mounts = append(mounts, DefaultContainerMounts...)
	mounts = append(mounts, specs.Mount{
		Source:      initBinPath,
		Destination: "/tmp/gdn-init",
		Type:        "bind",
		Options:     []string{"bind"},
	})
	// Following the current behaviour for privileged containers in Docker
	if privileged && privilegedMode == FullPrivilegedMode {
		for i, ociMount := range mounts {
			if ociMount.Destination == "/sys" || ociMount.Type == systemCgroupType() {
				clearReadOnly(&mounts[i])
			}
		}
	}
	return mounts
}

func clearReadOnly(m *specs.Mount) {
	var opt []string
	for _, o := range m.Options {
		if o != "ro" {
			opt = append(opt, o)
		}
	}
	m.Options = opt
}

func systemCgroupType() string {
	if cgroups.IsCgroup2UnifiedMode() {
		return "cgroup2"
	}
	return "cgroup"
}

func maskedPaths(privileged bool, privilegedMode PrivilegedMode) []string {
	if privileged && privilegedMode != IgnorePrivilegedMode {
		return nil
	}
	return config.DefaultMaskedPaths
}

func readOnlyPaths(privileged bool, privilegedMode PrivilegedMode) []string {
	if privileged && privilegedMode != IgnorePrivilegedMode {
		return nil
	}
	return config.DefaultReadOnlyPaths
}
