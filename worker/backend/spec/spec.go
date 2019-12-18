package spec

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"github.com/imdario/mergo"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// OciSpec converts a given `garden` container specification to an OCI spec.
//
// TODO
// - limits
// - masked paths
// - rootfs propagation
// - seccomp
// - user namespaces: uid/gid mappings
// x capabilities
// x devices
// x env
// x hostname
// x mounts
// x namespaces
// x rootfs
//
//
func OciSpec(gdn garden.ContainerSpec) (oci *specs.Spec, err error) {
	var (
		rootfs string
		mounts []specs.Mount
	)

	if gdn.Handle == "" {
		err = fmt.Errorf("handle must be specified")
		return
	}

	if gdn.RootFSPath == "" {
		gdn.RootFSPath = gdn.Image.URI
	}

	rootfs, err = rootfsDir(gdn.RootFSPath)
	if err != nil {
		return
	}

	mounts, err = OciSpecBindMounts(gdn.BindMounts)
	if err != nil {
		return
	}

	oci = merge(defaultGardenOciSpec(gdn.Privileged), &specs.Spec{
		Version:  specs.Version,
		Hostname: gdn.Handle,
		Process: &specs.Process{
			Env: gdn.Env,
		},
		Root:        &specs.Root{Path: rootfs},
		Mounts:      mounts,
		Annotations: map[string]string(gdn.Properties),
		// Linux: &specs.Linux{
		// 	Resources: &specs.LinuxResources{Memory: nil, Cpu: nil},
		// },
	})

	return
}

// OciSpecBindMounts converts garden bindmounts to oci spec mounts.
//
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

// defaultGardenOciSpec represents a default set of properties necessary in
// order to satisfy the garden interface.
//
// ps.: this spec is NOT completed - it must be merged with more properties to
// form a properly working container.
//
func defaultGardenOciSpec(privileged bool) *specs.Spec {
	var (
		namespaces   = OciNamespaces(privileged)
		capabilities = OciCapabilities(privileged)
	)

	return &specs.Spec{
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
		},
		Mounts: AnyContainerMounts,
	}
}

// merge merges an OCI spec `dst` into `src`.
//
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
//
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
