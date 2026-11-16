//go:build linux

package spec_test

import (
	"testing"

	"github.com/concourse/concourse/v8/worker/runtime/spec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DevicesSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *DevicesSuite) TestParseDeviceRule() {
	for _, tc := range []struct {
		desc     string
		input    string
		expected specs.LinuxDeviceCgroup
		succeeds bool
	}{
		{
			desc:  "block device with wildcard minor",
			input: "b 7:* rwm",
			expected: specs.LinuxDeviceCgroup{
				Allow: true, Type: "b", Major: int64Ptr(7), Minor: nil, Access: "rwm",
			},
			succeeds: true,
		},
		{
			desc:  "char device with specific major and minor",
			input: "c 10:237 rwm",
			expected: specs.LinuxDeviceCgroup{
				Allow: true, Type: "c", Major: int64Ptr(10), Minor: int64Ptr(237), Access: "rwm",
			},
			succeeds: true,
		},
		{
			desc:  "wildcard type with wildcard major and minor",
			input: "a *:* rwm",
			expected: specs.LinuxDeviceCgroup{
				Allow: true, Type: "a", Major: nil, Minor: nil, Access: "rwm",
			},
			succeeds: true,
		},
		{
			desc:  "read-only access",
			input: "c 1:3 r",
			expected: specs.LinuxDeviceCgroup{
				Allow: true, Type: "c", Major: int64Ptr(1), Minor: int64Ptr(3), Access: "r",
			},
			succeeds: true,
		},
		{
			desc:     "invalid format - missing parts",
			input:    "b 7:*",
			succeeds: false,
		},
		{
			desc:     "invalid device type",
			input:    "x 7:* rwm",
			succeeds: false,
		},
		{
			desc:     "invalid major number",
			input:    "b abc:* rwm",
			succeeds: false,
		},
		{
			desc:     "invalid minor number",
			input:    "b 7:abc rwm",
			succeeds: false,
		},
		{
			desc:     "invalid access character",
			input:    "b 7:* xyz",
			succeeds: false,
		},
		{
			desc:     "missing colon in major:minor",
			input:    "b 7 rwm",
			succeeds: false,
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			actual, err := spec.ParseDeviceRule(tc.input)
			if !tc.succeeds {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.expected, actual)
		})
	}
}

func (s *DevicesSuite) TestParseAllowedDevices() {
	devices, err := spec.ParseAllowedDevices([]string{"b 7:* rwm", "c 10:237 rwm"})
	s.NoError(err)
	s.Len(devices, 2)
	s.Equal("b", devices[0].Type)
	s.Equal("c", devices[1].Type)

	_, err = spec.ParseAllowedDevices([]string{"b 7:* rwm", "invalid"})
	s.Error(err)
}

func devicePaths(devices []specs.LinuxDevice) []string {
	paths := make([]string, 0, len(devices))
	for _, d := range devices {
		paths = append(paths, d.Path)
	}
	return paths
}

func (s *DevicesSuite) TestDevices() {
	for _, tc := range []struct {
		desc           string
		privilegedMode spec.PrivilegedMode
		privileged     bool
		expected       []string
	}{
		{
			desc:           "full privileged gets fuse only",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     true,
			expected:       []string{"/dev/fuse"},
		},
		{
			desc:           "fuse-only privileged gets fuse only",
			privilegedMode: spec.FUSEOnlyPrivilegedMode,
			privileged:     true,
			expected:       []string{"/dev/fuse"},
		},
		{
			desc:           "rootless privileged gets fuse and tun",
			privilegedMode: spec.RootlessPrivilegedMode,
			privileged:     true,
			expected:       []string{"/dev/fuse", "/dev/net/tun"},
		},
		{
			desc:           "ignore mode gets nothing even when privileged",
			privilegedMode: spec.IgnorePrivilegedMode,
			privileged:     true,
			expected:       []string{},
		},
		{
			desc:           "unprivileged gets nothing",
			privilegedMode: spec.FullPrivilegedMode,
			privileged:     false,
			expected:       []string{},
		},
	} {
		s.T().Run(tc.desc, func(_ *testing.T) {
			actual := spec.Devices(tc.privilegedMode, tc.privileged)
			s.ElementsMatch(tc.expected, devicePaths(actual))
		})
	}
}

func tunCgroupRuleAllowed(rules []specs.LinuxDeviceCgroup) bool {
	for _, rule := range rules {
		if rule.Type == "c" && rule.Major != nil && *rule.Major == 10 &&
			rule.Minor != nil && *rule.Minor == 200 {
			return rule.Allow && rule.Access == "rwm"
		}
	}
	return false
}
