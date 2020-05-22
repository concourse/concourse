package spec

import "github.com/opencontainers/runtime-spec/specs-go"

var (
	AnyContainerDevices = []specs.LinuxDeviceCgroup{
		// runc allows these
		{Access: "m", Type: "c", Major: deviceWildcard(), Minor: deviceWildcard(), Allow: true},
		{Access: "m", Type: "b", Major: deviceWildcard(), Minor: deviceWildcard(), Allow: true},

		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(3), Allow: true},          // /dev/null
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(8), Allow: true},          // /dev/random
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(7), Allow: true},          // /dev/full
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(0), Allow: true},          // /dev/tty
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(5), Allow: true},          // /dev/zero
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(9), Allow: true},          // /dev/urandom
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(1), Allow: true},          // /dev/console
		{Access: "rwm", Type: "c", Major: intRef(136), Minor: deviceWildcard(), Allow: true}, // /dev/pts/*
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(2), Allow: true},          // /dev/ptmx
		{Access: "rwm", Type: "c", Major: intRef(10), Minor: intRef(200), Allow: true},       // /dev/net/tun

		// we allow this
		{Access: "rwm", Type: "c", Major: intRef(10), Minor: intRef(229), Allow: true}, // /dev/fuse
	}

	PrivilegedOnlyDevices = []specs.LinuxDeviceCgroup{
		{Allow: false, Access: "rwm"},
	}
)

func intRef(i int64) *int64  { return &i }
func deviceWildcard() *int64 { return intRef(-1) }
