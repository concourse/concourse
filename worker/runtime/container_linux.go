//go:build linux

package runtime

import (
	"context"
	"regexp"
	"slices"

	"code.cloudfoundry.org/garden"
	containerd "github.com/containerd/containerd/v2/client"
)

const (
	SuperuserPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	Path          = "PATH=/usr/local/bin:/usr/bin:/bin"
)

func processCwd(dir string) string {
	if dir == "" {
		return "/"
	}
	return dir
}

var pathRegexp = regexp.MustCompile("^PATH=.*$")

// closeStdinAfterStart marks stdin closable for processes without a TTY, so
// runc closes the process's stdin once the client stream finishes.
//
// The reason we don't do this when there is a TTY is that runc signals such
// processes with SIGHUP when stdin is closed and we have called CloseIO
// (which doesn't actually close the stdin stream for the container - it just
// marks the stream as "closable").
//
// If we were to call CloseIO immediately on processes with a TTY, if the
// Stdin stream ever receives an error (e.g. an io.EOF due to worker
// rebalancing, or the worker restarting gracefully), runc will kill the
// process with SIGHUP (because we would have marked the stream as closable).
//
// Note: resource containers are the only ones without a TTY - task and
// hijack processes have a TTY enabled.
func closeStdinAfterStart(ctx context.Context, proc containerd.Process, spec garden.ProcessSpec) error {
	if spec.TTY != nil {
		return nil
	}

	return proc.CloseIO(ctx, containerd.WithStdinCloser)
}

// Set a default path based on the UID if no existing PATH is found
func envWithDefaultPath(uid uint32, currentEnv []string) string {
	pathFound := slices.ContainsFunc(currentEnv, pathRegexp.MatchString)
	if pathFound {
		return ""
	}

	if uid == 0 {
		return SuperuserPath
	}

	return Path
}
