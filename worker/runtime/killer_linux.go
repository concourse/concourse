//go:build linux

package runtime

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/api/types/runc/options"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/typeurl/v2"
)

// taskExecedProcesses retrieves a task's processes.
func taskExecedProcesses(ctx context.Context, task containerd.Task) ([]containerd.Process, error) {
	pids, err := task.Pids(context.Background())
	if err != nil {
		return nil, fmt.Errorf("pid listing: %w", err)
	}

	procs := []containerd.Process{}
	for _, pid := range pids {
		if pid.Info == nil { // init
			continue
		}

		// the protobuf message has a "catch-all" field for `pid.Info`,
		// thus, we need to unmarshal the message ourselves.
		//
		info, err := typeurl.UnmarshalAny(pid.Info)
		if err != nil {
			return nil, fmt.Errorf("proc details unmarshal: %w", err)
		}

		pinfo, ok := info.(*options.ProcessDetails)
		if !ok {
			return nil, fmt.Errorf("unknown proc detail type")
		}

		proc, err := task.LoadProcess(ctx, pinfo.ExecID, cio.Load)
		if err != nil {
			return nil, fmt.Errorf("load process: %w", err)
		}

		procs = append(procs, proc)
	}

	return procs, nil
}
