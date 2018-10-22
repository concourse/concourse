package version

import (
	"fmt"

	semver "github.com/cppforlife/go-semi-semantic/version"
)

func GetSemver(versionStr string) (major int, minor int, patch int, err error) {
	version, err := semver.NewVersionFromString(versionStr)
	if err != nil {
		return
	}

	if len(version.Release.Components) == 3 {
		major = version.Release.Components[0].(semver.VerSegCompInt).I
		minor = version.Release.Components[1].(semver.VerSegCompInt).I
		patch = version.Release.Components[2].(semver.VerSegCompInt).I
	} else {
		err = fmt.Errorf("Wrong number of components")
		return
	}

	return major, minor, patch, nil
}

func IsDev(versionStr string) bool {
	version, err := semver.NewVersionFromString(versionStr)
	if err != nil {
		return false
	}

	for _, item := range version.PreRelease.Components {
		if item.AsString() == "dev" {
			return true
		}
	}

	for _, item := range version.PostRelease.Components {
		if item.AsString() == "dev" {
			return true
		}
	}
	return false
}
