//go:build linux

package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"dario.cat/mergo"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type PrivilegedMode int

const (
	// Full privileged mode means that privileged tasks can do everything.
	// It is equivalent to full root on the host.
	FullPrivilegedMode PrivilegedMode = iota
	// Ignore privileged mode means that privileged tasks have the same
	// powers as unprivileged tasks (effectively disabling privileged).
	IgnorePrivilegedMode
	// FUSE-only privileged mode means that privileged tasks have access to
	// create and mount FUSE filesystems. This is enough to use
	// fuse-overlayfs, but not to escape the container.
	FUSEOnlyPrivilegedMode
)

func (pm *PrivilegedMode) UnmarshalFlag(value string) error {
	switch strings.ToLower(value) {
	case "full":
		*pm = FullPrivilegedMode
	case "ignore":
		*pm = IgnorePrivilegedMode
	case "fuse-only":
		*pm = FUSEOnlyPrivilegedMode
	default:
		return fmt.Errorf("unsupported value for PrivilegedMode: %s", value)
	}
	return nil
}

const baseCgroupsPath = "garden"

var isSwapLimitEnabled bool

func init() {
	isSwapLimitEnabled = swapLimitEnabled()
}

func swapLimitEnabled() bool {
	swapLimitFile := "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"
	_, err := os.Stat(swapLimitFile)
	return err == nil
}

// OciSpec converts a given `garden` container specification to an OCI spec.
func OciSpec(initBinPath string, seccomp specs.LinuxSeccomp, seccompFuse specs.LinuxSeccomp, hooks specs.Hooks, privilegedMode PrivilegedMode, gdn garden.ContainerSpec, maxUid, maxGid uint32) (oci *specs.Spec, err error) {
	if gdn.Handle == "" {
		err = fmt.Errorf("handle must be specified")
		return
	}

	if gdn.RootFSPath == "" {
		gdn.RootFSPath = gdn.Image.URI
	}

	var rootfs string
	rootfs, err = rootfsDir(gdn.RootFSPath)
	if err != nil {
		return
	}

	var mounts []specs.Mount
	mounts, err = OciSpecBindMounts(gdn.BindMounts)
	if err != nil {
		return
	}

	resources := OciResources(gdn.Limits, isSwapLimitEnabled)
	cgroupsPath := OciCgroupsPath(baseCgroupsPath, gdn.Handle, privilegedMode, gdn.Privileged)

	oci = merge(
		defaultGardenOciSpec(initBinPath, seccomp, seccompFuse, privilegedMode, gdn.Privileged, maxUid, maxGid),
		&specs.Spec{
			Version:  specs.Version,
			Hostname: gdn.Handle,
			Process: &specs.Process{
				Env: gdn.Env,
			},
			Hooks:       &hooks,
			Root:        &specs.Root{Path: rootfs},
			Mounts:      mounts,
			Annotations: map[string]string(gdn.Properties),
			Linux: &specs.Linux{
				Resources:   resources,
				CgroupsPath: cgroupsPath,
			},
		},
	)

	return oci, nil
}

// OciSpecBindMounts converts garden bindmounts to oci spec mounts.
func OciSpecBindMounts(bindMounts []garden.BindMount) (mounts []specs.Mount, err error) {
	for _, bindMount := range bindMounts {
		if bindMount.SrcPath == "" || bindMount.DstPath == "" {
			err = fmt.Errorf("src and dst must not be empty")
			return
		}

		if !filepath.IsAbs(bindMount.SrcPath) || !filepath.IsAbs(bindMount.DstPath) {
			err = fmt.Errorf("src and dst must be absolute")
			return
		}

		if bindMount.Origin != garden.BindMountOriginHost {
			err = fmt.Errorf("unknown bind mount origin %d", bindMount.Origin)
			return
		}

		mode := "ro"
		switch bindMount.Mode {
		case garden.BindMountModeRO:
		case garden.BindMountModeRW:
			mode = "rw"
		default:
			err = fmt.Errorf("unknown bind mount mode %d", bindMount.Mode)
			return
		}

		mounts = append(mounts, specs.Mount{
			Source:      bindMount.SrcPath,
			Destination: bindMount.DstPath,
			Type:        "bind",
			Options:     []string{"bind", mode},
		})
	}

	return
}

// OciIDMappings provides the uid/gid mappings for user namespaces (if
// necessary, based on `privileged`).
func OciIDMappings(privilegedMode PrivilegedMode, privileged bool, max uint32) []specs.LinuxIDMapping {
	if privileged && privilegedMode == FullPrivilegedMode {
		return []specs.LinuxIDMapping{}
	}

	return []specs.LinuxIDMapping{
		{ // "root" inside, but non-root outside
			ContainerID: 0,
			HostID:      max,
			Size:        1,
		},
		{ // anything else, not root inside & outside
			ContainerID: 1,
			HostID:      1,
			Size:        max - 1,
		},
	}
}

func OciResources(limits garden.Limits, swapLimitEnabled bool) *specs.LinuxResources {
	var (
		cpuResources    *specs.LinuxCPU
		memoryResources *specs.LinuxMemory
		pidLimit        *specs.LinuxPids
	)
	shares := limits.CPU.LimitInShares
	if limits.CPU.Weight > 0 {
		shares = limits.CPU.Weight
	}

	if shares > 0 {
		cpuResources = &specs.LinuxCPU{
			Shares: &shares,
		}
	}

	memoryLimit := int64(limits.Memory.LimitInBytes)
	if memoryLimit > 0 {
		memoryResources = &specs.LinuxMemory{
			Limit: &memoryLimit,
		}
		if swapLimitEnabled {
			memoryResources.Swap = &memoryLimit
		}
	}

	maxPids := int64(limits.Pid.Max)
	if maxPids > 0 {
		pidLimit = &specs.LinuxPids{
			Limit: maxPids,
		}
	}

	if cpuResources == nil && memoryResources == nil && pidLimit == nil {
		return nil
	}
	return &specs.LinuxResources{
		CPU:    cpuResources,
		Memory: memoryResources,
		Pids:   pidLimit,
	}
}

func OciCgroupsPath(basePath, handle string, privilegedMode PrivilegedMode, privileged bool) string {
	if privilegedMode == FullPrivilegedMode && privileged {
		return ""
	}
	return filepath.Join(basePath, handle)
}

// defaultGardenOciSpec represents a default set of properties necessary in
// order to satisfy the garden interface.
//
// ps.: this spec is NOT completed - it must be merged with more properties to
// form a properly working container.
func defaultGardenOciSpec(initBinPath string, seccomp specs.LinuxSeccomp, seccompFuse specs.LinuxSeccomp, privilegedMode PrivilegedMode, privileged bool, maxUid, maxGid uint32) *specs.Spec {
	var (
		namespaces   = OciNamespaces(privilegedMode, privileged)
		capabilities = OciCapabilities(privilegedMode, privileged)
	)

	spec := &specs.Spec{
		Process: &specs.Process{
			Args:         []string{"/tmp/gdn-init"},
			Capabilities: &capabilities,
			Cwd:          "/",
		},
		Linux: &specs.Linux{
			Namespaces: namespaces,
			Resources: &specs.LinuxResources{
				Devices: AnyContainerDevices,
			},
			Devices:     Devices(privilegedMode, privileged),
			UIDMappings: OciIDMappings(privilegedMode, privileged, maxUid),
			GIDMappings: OciIDMappings(privilegedMode, privileged, maxGid),
		},
		Mounts: ContainerMounts(privilegedMode, privileged, initBinPath),
	}

	if !privileged || privilegedMode != FullPrivilegedMode {
		if privilegedMode == FUSEOnlyPrivilegedMode {
			spec.Linux.Seccomp = &seccompFuse
		} else {
			spec.Linux.Seccomp = &seccomp
		}
	}

	return spec
}

// merge merges an OCI spec `dst` into `src`.
func merge(dst, src *specs.Spec) *specs.Spec {
	err := mergo.Merge(dst, src, mergo.WithAppendSlice)
	if err != nil {
		panic(fmt.Errorf(
			"failed to merge specs %v %v - programming mistake? %w",
			dst, src, err,
		))
	}

	return dst
}

// rootfsDir takes a raw rootfs uri and extracts the directory that it points to,
// if using a valid scheme (`raw://`)
func rootfsDir(raw string) (directory string, err error) {
	if raw == "" {
		err = fmt.Errorf("rootfs must not be empty")
		return
	}

	parts := strings.SplitN(raw, "://", 2)
	if len(parts) != 2 {
		err = fmt.Errorf("malformatted rootfs: must be of form 'scheme://<abs_dir>'")
		return
	}

	var scheme string
	scheme, directory = parts[0], parts[1]
	if scheme != "raw" {
		err = fmt.Errorf("unsupported scheme '%s'", scheme)
		return
	}

	if !filepath.IsAbs(directory) {
		err = fmt.Errorf("directory must be an absolute path")
		return
	}

	return
}
