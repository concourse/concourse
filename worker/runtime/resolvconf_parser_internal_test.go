//go:build linux

package runtime

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHostResolveConf_UsesSystemdResolvedWhenHostResolvConfIsLoopback(t *testing.T) {
	oldReadFileFn := readFileFn
	oldLocalIPFn := localIPFn
	defer func() {
		readFileFn = oldReadFileFn
		localIPFn = oldLocalIPFn
	}()

	readFileFn = func(path string) ([]byte, error) {
		switch path {
		case defaultHostResolvConfPath:
			return []byte("nameserver 127.0.0.53\noptions edns0 trust-ad\nsearch .\n"), nil
		case systemdResolvedResolvConfPath:
			return []byte("nameserver 169.254.0.2\nsearch .\n"), nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", path)
		}
	}

	localIPFn = func() (string, error) {
		return "10.5.6.11", nil
	}

	entries, err := ParseHostResolveConf(defaultHostResolvConfPath)
	require.NoError(t, err)
	require.Equal(t, []string{"nameserver 169.254.0.2", "search ."}, entries)
}

func TestParseHostResolveConf_FallsBackToLocalIPWhenSystemdResolvedUnavailable(t *testing.T) {
	oldReadFileFn := readFileFn
	oldLocalIPFn := localIPFn
	defer func() {
		readFileFn = oldReadFileFn
		localIPFn = oldLocalIPFn
	}()

	readFileFn = func(path string) ([]byte, error) {
		switch path {
		case defaultHostResolvConfPath:
			return []byte("nameserver 127.0.0.53\noptions edns0 trust-ad\nsearch .\n"), nil
		case systemdResolvedResolvConfPath:
			return nil, fmt.Errorf("not found")
		default:
			return nil, fmt.Errorf("unexpected path: %s", path)
		}
	}

	localIPFn = func() (string, error) {
		return "10.5.6.11", nil
	}

	entries, err := ParseHostResolveConf(defaultHostResolvConfPath)
	require.NoError(t, err)
	require.Equal(t, []string{"nameserver 10.5.6.11"}, entries)
}
