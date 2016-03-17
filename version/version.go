package version

import (
	"strconv"
	"strings"
)

// overridden via linker flags
var Version = "0.0.0-dev"

func GetSemver(versionStr string) (major int, minor int, patch int, err error) {
	parts := strings.SplitN(versionStr, ".", 3)

	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return major, minor, patch, err
		}
	}
	if len(parts) >= 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return major, minor, patch, err
		}
	}
	if len(parts) >= 1 {
		major, err = strconv.Atoi(parts[0])
		if err != nil {
			return major, minor, patch, err
		}
	}
	return major, minor, patch, nil
}

func IsDev(versionStr string) bool {
	return strings.HasSuffix(versionStr, "-dev")
}
