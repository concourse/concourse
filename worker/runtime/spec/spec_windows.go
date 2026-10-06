//go:build windows

package spec

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const DefaultInitBinPath = `C:\concourse\bin\init.exe`

// InitBinContainerDir is where the directory holding the init executable gets
// mounted inside every container.
const InitBinContainerDir = `C:\concourse\bin`

// OciSpec converts a given `garden` container specification to a Windows OCI
// spec. The seccomp profiles, OCI hooks, privileged mode, user namespace
// mappings and device cgroup rules are Linux-only concepts and are ignored.
//
// The rootfs directory must contain a valid Windows container layer.
func OciSpec(initBinPath string, seccomp specs.LinuxSeccomp, seccompFuse specs.LinuxSeccomp, hooks specs.Hooks, privilegedMode PrivilegedMode, gdn garden.ContainerSpec, maxUid, maxGid uint32, extraDevices ...[]specs.LinuxDeviceCgroup) (oci *specs.Spec, err error) {
	if gdn.Handle == "" {
		return nil, fmt.Errorf("handle must be specified")
	}

	if gdn.RootFSPath == "" {
		gdn.RootFSPath = gdn.Image.URI
	}

	mounts, err := OciSpecBindMounts(gdn.BindMounts)
	if err != nil {
		return nil, err
	}

	// The init executable's directory is mounted into the container and the
	// init process is kept running so the container stays alive until
	// Concourse decides it's time to exec the actual command.
	mounts = append(mounts, specs.Mount{
		Source:      filepath.Dir(initBinPath),
		Destination: InitBinContainerDir,
		Options:     []string{"ro"},
	})

	oci = &specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{filepath.Join(InitBinContainerDir, filepath.Base(initBinPath))},
			Cwd:  `C:\`,
			Env:  gdn.Env,
		},
		Mounts:      mounts,
		Annotations: map[string]string(gdn.Properties),
		Windows: &specs.Windows{
			Resources:               OciWindowsResources(gdn.Limits),
			IgnoreFlushesDuringBoot: true,
		},
	}

	if imageRef, ok := strings.CutPrefix(gdn.RootFSPath, OCIImageScheme+"://"); ok {
		// the image is pulled natively through containerd at container
		// creation; the snapshotter provides the rootfs
		if oci.Annotations == nil {
			oci.Annotations = map[string]string{}
		}
		oci.Annotations[OCIImageAnnotation] = imageRef

		return oci, nil
	}

	rootfs, err := rootfsDir(gdn.RootFSPath)
	if err != nil {
		return nil, err
	}

	oci.Root = &specs.Root{Path: rootfs}
	oci.Windows.LayerFolders = []string{rootfs}

	return oci, nil
}

// WindowsContainerPath converts the POSIX-style absolute container paths the
// web node uses (e.g. /tmp/build/abc) to Windows container paths on the
// system drive. Paths that are already Windows-style are left alone.
func WindowsContainerPath(p string) string {
	if strings.HasPrefix(p, "/") {
		return "C:" + filepath.FromSlash(path.Clean(p))
	}
	return p
}

// OciSpecBindMounts converts garden bindmounts to oci spec mounts.
func OciSpecBindMounts(bindMounts []garden.BindMount) (mounts []specs.Mount, err error) {
	for _, bindMount := range bindMounts {
		if bindMount.SrcPath == "" || bindMount.DstPath == "" {
			return nil, fmt.Errorf("src and dst must not be empty")
		}

		dstPath := WindowsContainerPath(bindMount.DstPath)

		if !filepath.IsAbs(bindMount.SrcPath) || !filepath.IsAbs(dstPath) {
			return nil, fmt.Errorf("src and dst must be absolute")
		}

		if bindMount.Origin != garden.BindMountOriginHost {
			return nil, fmt.Errorf("unknown bind mount origin %d", bindMount.Origin)
		}

		mode := "ro"
		switch bindMount.Mode {
		case garden.BindMountModeRO:
		case garden.BindMountModeRW:
			mode = "rw"
		default:
			return nil, fmt.Errorf("unknown bind mount mode %d", bindMount.Mode)
		}

		mounts = append(mounts, specs.Mount{
			Source:      bindMount.SrcPath,
			Destination: dstPath,
			Options:     []string{mode},
		})
	}

	return mounts, nil
}

func OciWindowsResources(limits garden.Limits) *specs.WindowsResources {
	var (
		cpuResources    *specs.WindowsCPUResources
		memoryResources *specs.WindowsMemoryResources
	)

	shares := limits.CPU.LimitInShares
	if limits.CPU.Weight > 0 {
		shares = limits.CPU.Weight
	}

	if shares > 0 {
		windowsShares := uint16(shares)
		cpuResources = &specs.WindowsCPUResources{
			Shares: &windowsShares,
		}
	}

	memoryLimit := limits.Memory.LimitInBytes
	if memoryLimit > 0 {
		memoryResources = &specs.WindowsMemoryResources{
			Limit: &memoryLimit,
		}
	}

	if cpuResources == nil && memoryResources == nil {
		return nil
	}

	return &specs.WindowsResources{
		CPU:    cpuResources,
		Memory: memoryResources,
	}
}
