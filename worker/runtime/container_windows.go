//go:build windows

package runtime

import (
	"context"

	"code.cloudfoundry.org/garden"
	bespec "github.com/concourse/concourse/v8/worker/runtime/spec"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
)

// closeStdinAfterStart is a no-op on Windows: the runhcs shim closes the
// process's stdin as soon as CloseIO is called - before the client's stdin
// payload has necessarily been copied in - so processes reading stdin (like
// resource checks) would see an immediate EOF. The shim closes stdin by
// itself once the worker's stdin pipe reaches EOF.
func closeStdinAfterStart(ctx context.Context, proc containerd.Process, spec garden.ProcessSpec) error {
	return nil
}

func processCwd(dir string) string {
	if dir == "" {
		return `C:\`
	}
	return bespec.WindowsContainerPath(dir)
}

// processIOCreator names a process's stdio pipes after its container as well
// as its ID. containerd names them after the ID alone and pipe names are
// global on Windows, so two containers that each run a process with the same
// ID, as every task step does, would otherwise claim the same pipes.
func processIOCreator(handle string, opts []cio.Opt) cio.Creator {
	creator := cio.NewCreator(opts...)
	return func(id string) (cio.IO, error) {
		return creator(handle + "-" + id)
	}
}

// envWithDefaultPath never injects a PATH on Windows; the image's own
// environment applies.
func envWithDefaultPath(uid uint32, currentEnv []string) string {
	return ""
}
