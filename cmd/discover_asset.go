package cmd

import (
	"os"
	"path/filepath"
)

// discoverAsset will find an asset path relative to the executable, assuming
// the executable is installed as /usr/local/concourse/bin/concourse, and the
// asset lives under /usr/local/concourse
func DiscoverAsset(name string) string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}

	asset := filepath.Join(filepath.Dir(filepath.Dir(self)), name)
	if _, err := os.Stat(asset); err == nil {
		return asset
	}

	return ""
}
