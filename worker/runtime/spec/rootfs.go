package spec

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// OCIImageScheme is the rootfs URI scheme that makes the worker pull the
// container image natively through containerd instead of using a streamed
// rootfs volume, e.g. in a task config:
//
//	rootfs_uri: oci://mcr.microsoft.com/windows/servercore:ltsc2022
const OCIImageScheme = "oci"

// OCIImageAnnotation carries the native image reference through the OCI spec
// to container creation.
const OCIImageAnnotation = "org.concourse.oci-image"

// rootfsDir takes a raw rootfs uri and extracts the directory that it points to,
// if using a valid scheme (`raw://`)
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
		// the web node builds this URI with net/url, which percent-encodes
		// Windows path separators (raw://C:%5C...)
		unescaped, uerr := url.PathUnescape(directory)
		if uerr != nil || !filepath.IsAbs(unescaped) {
			err = fmt.Errorf("directory must be an absolute path")
			return
		}
		directory = filepath.Clean(unescaped)
	}

	return
}
