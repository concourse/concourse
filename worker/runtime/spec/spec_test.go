//go:build linux

package spec_test

import (
	"testing"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime/spec"
	"github.com/containers/common/pkg/config"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	dummyMaxUid uint32 = 0
	dummyMaxGid uint32 = 0
)

type SpecSuite struct {
	suite.Suite
	*require.Assertions
}

func uint64Ptr(i uint64) *uint64 { return &i }
func int64Ptr(i int64) *int64    { return &i }

func (s *SpecSuite) TestContainerSpecValidations() {
	for _, tc := range []struct {
		desc string
		spec garden.ContainerSpec
	}{
		{
			desc: "no handle specified",
			spec: garden.ContainerSpec{},
		},
		{
			desc: "rootfsPath not specified",
			spec: garden.ContainerSpec{
				Handle: "handle",
			},
		},
		{
			desc: "rootfsPath without scheme",
			spec: garden.ContainerSpec{
				Handle:     "handle",
				RootFSPath: "foo",
			},
		},
		{
			desc: "rootfsPath with unknown scheme",
			spec: garden.ContainerSpec{
				Handle:     "handle",
				RootFSPath: "weird://foo",
			},
		},
		{
			desc: "rootfsPath not being absolute",
			spec: garden.ContainerSpec{
				Handle:     "handle",
				RootFSPath: "raw://../not/absolute/at/all",
			},
		},
		{
			desc: "both rootfsPath and image specified",
			spec: garden.ContainerSpec{
				Handle:     "handle",
				RootFSPath: "foo",
				Image:      garden.ImageRef{URI: "bar"},
			},
		},
		{
			desc: "no rootfsPath, but image specified w/out scheme",
			spec: garden.ContainerSpec{
				Handle: "handle",
				Image:  garden.ImageRef{URI: "bar"},
			},
		},
		{
			desc: "no rootfsPath, but image specified w/ unknown scheme",
			spec: garden.ContainerSpec{
				Handle: "handle",
				Image:  garden.ImageRef{URI: "weird://bar"},
			},
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			_, err := spec.OciSpec(spec.DefaultInitBinPath, spec.GetDefaultSeccompProfile(), spec.GetDefaultSeccompProfileFuse(), specs.Hooks{}, spec.FullPrivilegedMode, tc.spec, dummyMaxUid, dummyMaxGid)
			s.Error(err)
		})
	}
}

func (s *SpecSuite) TestIDMappings() {
	// TODO
	//
	// ensure that we mutate the right thing
}

func (s *SpecSuite) TestOciSpecBindMounts() {
	for _, tc := range []struct {
		desc     string
		mounts   []garden.BindMount
		expected []specs.Mount
		succeeds bool
	}{
		{
			desc:     "unknown mode",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					SrcPath: "/a",
					DstPath: "/b",
					Mode:    123,
					Origin:  garden.BindMountOriginHost,
				},
			},
		},
		{
			desc:     "unknown origin",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					SrcPath: "/a",
					DstPath: "/b",
					Mode:    garden.BindMountModeRO,
					Origin:  123,
				},
			},
		},
		{
			desc:     "w/out src",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					DstPath: "/b",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				},
			},
		},
		{
			desc:     "non-absolute src",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					DstPath: "/b",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				},
			},
		},
		{
			desc:     "w/out dest",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					SrcPath: "/a",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				},
			},
		},
		{
			desc:     "non-absolute dest",
			succeeds: false,
			mounts: []garden.BindMount{
				{
					DstPath: "/b",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				},
			},
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			actual, err := spec.OciSpecBindMounts(tc.mounts)
			if !tc.succeeds {
				s.Error(err)
				return
			}

			s.NoError(err)
			s.Equal(tc.expected, actual)
		})
	}
}

func (s *SpecSuite) TestOciNamespaces() {
	for _, tc := range []struct {
		desc           string
		privilegedMode spec.PrivilegedMode
		privileged     bool
		expected       []specs.LinuxNamespace
	}{
		{
			desc:           "privileged - full",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     true,
			expected:       spec.PrivilegedContainerNamespaces,
		},
		{
			desc:           "unprivileged - full",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     false,
			expected:       spec.UnprivilegedContainerNamespaces,
		},
		{
			desc:           "privileged - FUSE-only",
			privilegedMode: spec.FUSEOnlyPrivilegedMode,
			privileged:     true,
			expected:       spec.UnprivilegedContainerNamespaces,
		},
		{
			desc:           "privileged - ignore",
			privilegedMode: spec.IgnorePrivilegedMode,
			privileged:     true,
			expected:       spec.UnprivilegedContainerNamespaces,
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.Equal(tc.expected, spec.OciNamespaces(tc.privilegedMode, tc.privileged))
		})
	}
}

func (s *SpecSuite) TestOciCapabilities() {
	for _, tc := range []struct {
		desc           string
		privileged     bool
		privilegedMode spec.PrivilegedMode
		expected       specs.LinuxCapabilities
	}{
		{
			desc:           "privileged FullPrivilegedMode",
			privileged:     true,
			privilegedMode: spec.FullPrivilegedMode,
			expected:       spec.PrivilegedContainerCapabilities,
		},
		{
			desc:           "privileged FUSEOnlyPrivilegedMode",
			privileged:     true,
			privilegedMode: spec.FUSEOnlyPrivilegedMode,
			expected:       spec.FUSEOnlyContainerCapabilities,
		},
		{
			desc:           "privileged IgnorePrivilegedMode",
			privileged:     true,
			privilegedMode: spec.IgnorePrivilegedMode,
			expected:       spec.UnprivilegedContainerCapabilities,
		},
		{
			desc:       "unprivileged",
			privileged: false,
			expected:   spec.UnprivilegedContainerCapabilities,
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.Equal(tc.expected, spec.OciCapabilities(tc.privilegedMode, tc.privileged))
		})
	}
}

func (s *SpecSuite) TestOciResourceLimits() {
	for _, tc := range []struct {
		desc        string
		limits      garden.Limits
		expected    *specs.LinuxResources
		swapEnabled bool
	}{
		{
			desc: "CPU limit in weight",
			limits: garden.Limits{
				CPU: garden.CPULimits{
					Weight: 512,
				},
			},
			expected: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Shares: uint64Ptr(512),
				},
			},
			swapEnabled: true,
		},
		{
			desc: "CPU limit in shares",
			limits: garden.Limits{
				CPU: garden.CPULimits{
					LimitInShares: 512,
				},
			},
			expected: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Shares: uint64Ptr(512),
				},
			},
			swapEnabled: true,
		},
		{
			desc: "CPU limit prefers weight",
			limits: garden.Limits{
				CPU: garden.CPULimits{
					LimitInShares: 512,
					Weight:        1024,
				},
			},
			expected: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Shares: uint64Ptr(1024),
				},
			},
			swapEnabled: true,
		},
		{
			desc: "Memory limit",
			limits: garden.Limits{
				Memory: garden.MemoryLimits{
					LimitInBytes: 10000,
				},
			},
			expected: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: int64Ptr(10000),
					Swap:  int64Ptr(10000),
				},
			},
			swapEnabled: true,
		},
		{
			desc: "Swap memory limit disabled",
			limits: garden.Limits{
				Memory: garden.MemoryLimits{
					LimitInBytes: 10000,
				},
			},
			expected: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: int64Ptr(10000),
				},
			},
			swapEnabled: false,
		},
		{
			desc: "PID limit",
			limits: garden.Limits{
				Pid: garden.PidLimits{
					Max: 1000,
				},
			},
			expected: &specs.LinuxResources{
				Pids: &specs.LinuxPids{
					Limit: 1000,
				},
			},
			swapEnabled: true,
		},
		{
			desc:     "No limits specified",
			limits:   garden.Limits{},
			expected: nil,
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.Equal(tc.expected, spec.OciResources(tc.limits, tc.swapEnabled))
		})
	}
}

func (s *SpecSuite) TestOciCgroupsPath() {
	for _, tc := range []struct {
		desc           string
		basePath       string
		handle         string
		privilegedMode spec.PrivilegedMode
		privileged     bool
		expected       string
	}{
		{
			desc:           "not privileged",
			basePath:       "garden",
			handle:         "1234",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     false,
			expected:       "garden/1234",
		},
		{
			desc:           "privileged - full",
			basePath:       "garden",
			handle:         "1234",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     true,
			expected:       "",
		},
		{
			desc:           "privileged - FUSE-only",
			basePath:       "garden",
			handle:         "1234",
			privilegedMode: spec.FUSEOnlyPrivilegedMode,
			privileged:     true,
			expected:       "garden/1234",
		},
		{
			desc:           "privileged - ignore",
			basePath:       "garden",
			handle:         "1234",
			privilegedMode: spec.IgnorePrivilegedMode,
			privileged:     true,
			expected:       "garden/1234",
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			s.Equal(tc.expected, spec.OciCgroupsPath(tc.basePath, tc.handle, tc.privilegedMode, tc.privileged))
		})
	}
}

func (s *SpecSuite) TestContainerSpec() {
	cgroupType := "cgroup"
	if cgroups.IsCgroup2UnifiedMode() {
		cgroupType = "cgroup2"
	}
	var minimalContainerSpec = garden.ContainerSpec{
		Handle: "handle", RootFSPath: "raw:///rootfs",
	}

	for _, tc := range []struct {
		desc           string
		gdn            garden.ContainerSpec
		privilegedMode spec.PrivilegedMode
		check          func(*specs.Spec)
	}{
		{
			desc: "defaults",
			gdn:  minimalContainerSpec,
			check: func(oci *specs.Spec) {
				s.Equal("/", oci.Process.Cwd)
				s.Equal([]string{"/tmp/gdn-init"}, oci.Process.Args)
				s.Equal(oci.Mounts, spec.ContainerMounts(spec.FullPrivilegedMode, false, spec.DefaultInitBinPath))
				s.Equal(config.DefaultMaskedPaths, oci.Linux.MaskedPaths)
				s.Equal(config.DefaultReadOnlyPaths, oci.Linux.ReadonlyPaths)

				s.Equal("/tmp/gdn-init", oci.Mounts[len(oci.Mounts)-1].Destination,
					"gdn-init mount should be mounted after all the other default mounts")

				s.Equal(minimalContainerSpec.Handle, oci.Hostname)
				s.Equal(spec.AnyContainerDevices, oci.Linux.Resources.Devices)
			},
		},
		{
			desc: "privileged mounts",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: true,
			},
			check: func(oci *specs.Spec) {
				s.Contains(oci.Mounts, specs.Mount{
					Destination: "/sys",
					Type:        "sysfs",
					Source:      "sysfs",
					Options:     []string{"nosuid", "noexec", "nodev"},
				})
				s.Contains(oci.Mounts, specs.Mount{
					Destination: "/sys/fs/cgroup",
					Type:        cgroupType,
					Source:      cgroupType,
					Options:     []string{"nosuid", "noexec", "nodev"},
				})
				for _, ociMount := range oci.Mounts {
					if ociMount.Destination == "/sys" {
						s.NotContains(ociMount.Options, "ro", "%s: %s", ociMount.Destination, ociMount.Type)
					} else if ociMount.Destination == "/sys/fs/cgroup" {
						s.NotContains(ociMount.Options, "ro", "%s: %s", ociMount.Destination, ociMount.Type)
					}
				}
				s.Empty(oci.Linux.MaskedPaths)
				s.Empty(oci.Linux.ReadonlyPaths)
			},
		},
		{
			desc: "privileged mounts with IgnorePrivilegedMode",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: true,
			},
			privilegedMode: spec.IgnorePrivilegedMode,
			check: func(oci *specs.Spec) {
				s.NotContains(oci.Mounts, specs.Mount{
					Destination: "/sys",
					Type:        "sysfs",
					Source:      "sysfs",
					Options:     []string{"nosuid", "noexec", "nodev"},
				})
				s.NotContains(oci.Mounts, specs.Mount{
					Destination: "/sys/fs/cgroup",
					Type:        cgroupType,
					Source:      cgroupType,
					Options:     []string{"nosuid", "noexec", "nodev"},
				})
				for _, ociMount := range oci.Mounts {
					if ociMount.Destination == "/sys" {
						s.Contains(ociMount.Options, "ro", "%s: %s", ociMount.Destination, ociMount.Type)
					} else if ociMount.Destination == "/sys/fs/cgroup" {
						s.Contains(ociMount.Options, "ro", "%s: %s", ociMount.Destination, ociMount.Type)
					}
				}
				s.Equal(config.DefaultMaskedPaths, oci.Linux.MaskedPaths)
				s.Equal(config.DefaultReadOnlyPaths, oci.Linux.ReadonlyPaths)
			},
		},
		{
			desc: "env with path already configured",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Env: []string{"foo=bar", "PATH=/somewhere"},
			},
			check: func(oci *specs.Spec) {
				s.Equal([]string{"foo=bar", "PATH=/somewhere"}, oci.Process.Env)
			},
		},
		{
			desc: "mounts",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				BindMounts: []garden.BindMount{
					{ // ro mount
						SrcPath: "/a",
						DstPath: "/b",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					},
					{ // rw mount
						SrcPath: "/a",
						DstPath: "/b",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginHost,
					},
				},
			},
			check: func(oci *specs.Spec) {
				s.Contains(oci.Mounts, specs.Mount{
					Source:      "/a",
					Destination: "/b",
					Type:        "bind",
					Options:     []string{"bind", "ro"},
				})
				s.Contains(oci.Mounts, specs.Mount{
					Source:      "/a",
					Destination: "/b",
					Type:        "bind",
					Options:     []string{"bind", "rw"},
				})
			},
		},
		{
			desc: "seccomp is not empty for unprivileged",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: false,
			},
			check: func(oci *specs.Spec) {
				s.NotEmpty(oci.Linux.Seccomp)
				s.NotContains(oci.Linux.Seccomp.Syscalls, spec.AllowSyscall("unshare"))
			},
		},
		{
			desc: "seccomp allows unshare loading for FUSE-only",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: true,
			},
			privilegedMode: spec.FUSEOnlyPrivilegedMode,
			check: func(oci *specs.Spec) {
				s.Contains(oci.Linux.Seccomp.Syscalls, spec.AllowSyscall("unshare"))
			},
		},
		{
			desc: "seccomp is empty for privileged",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: true,
			},
			check: func(oci *specs.Spec) {
				s.Empty(oci.Linux.Seccomp)
			},
		},
		{
			desc: "limits",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Limits: garden.Limits{
					CPU: garden.CPULimits{
						Weight: 512,
					},
					Memory: garden.MemoryLimits{
						LimitInBytes: 10000,
					},
					Pid: garden.PidLimits{
						Max: 1000,
					},
				},
			},
			check: func(oci *specs.Spec) {
				s.NotNil(oci.Linux.Resources.CPU)
				s.Equal(uint64Ptr(512), oci.Linux.Resources.CPU.Shares)
				s.NotNil(oci.Linux.Resources.Memory)
				s.Equal(int64Ptr(10000), oci.Linux.Resources.Memory.Limit)
				s.NotNil(oci.Linux.Resources.Pids)
				s.Equal(int64(1000), oci.Linux.Resources.Pids.Limit)

				s.NotNil(oci.Linux.Resources.Devices)
			},
		},
		{
			desc: "cgroups path",
			gdn: garden.ContainerSpec{
				Handle: "handle", RootFSPath: "raw:///rootfs",
				Privileged: false,
			},
			check: func(oci *specs.Spec) {
				s.Equal("garden/handle", oci.Linux.CgroupsPath)
			},
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			actual, err := spec.OciSpec(spec.DefaultInitBinPath, spec.GetDefaultSeccompProfile(), spec.GetDefaultSeccompProfileFuse(), specs.Hooks{}, tc.privilegedMode, tc.gdn, dummyMaxUid, dummyMaxGid)
			s.NoError(err)

			tc.check(actual)
		})
	}
}
