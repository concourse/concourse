package exec

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Takes a path and splits it into an artifact name and file path by splitting
// on the first '/'.
// e.g. some-artifact/path/to/file.json will return:
// some-artifact, path/to/file.json
func parseArtifactPath(path string) (artifactName, file string, err error) {
	if filepath.IsAbs(path) {
		path = strings.TrimLeft(path, "/")
	}

	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 {
		return "", "", fmt.Errorf("path '%s' does not specify the input volume where the file lives", path)
	}

	artifactName = segs[0]
	file = strings.TrimLeft(filepath.Clean("/"+segs[1]), "/")
	if file == "" {
		return "", "", fmt.Errorf("path '%s' does not specify a file", path)
	}

	return artifactName, file, nil
}
