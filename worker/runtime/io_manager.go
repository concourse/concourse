//go:build linux

package runtime

import (
	"sync"

	"github.com/containerd/containerd/v2/pkg/cio"
)

//counterfeiter:generate github.com/containerd/containerd/v2/pkg/cio.IO
//counterfeiter:generate . IOManager

type IOManager interface {
	Creator(containerID, taskID string, creator cio.Creator) cio.Creator
	Attach(containerID, taskID string, attach cio.Attach) cio.Attach
	Delete(containerID string)
	Get(containerID, taskID string) (cio.IO, bool)
}

// IOManager keeps track of the Readers that are attached to containerd Task
// FIFO files. IOManager allows us to guarantee that no more than one reader is
// attached to the FIFO files. If multiple readers are attached to the FIFO
// files, the stdout/stderr from a task will get split between all attached
// readers, resulting in us losing logs. The containerd docs state that the
// client (that's us!) have to ensure only one reader is attached to the task
// FIFO files.
type ioManager struct {
	ioReaders map[string]map[string]cio.IO
	lock      sync.Mutex
}

func NewIOManager() IOManager {
	return &ioManager{
		ioReaders: map[string]map[string]cio.IO{},
		lock:      sync.Mutex{},
	}
}

func (i *ioManager) Creator(containerID, taskID string, creator cio.Creator) cio.Creator {
	return func(id string) (cio.IO, error) {
		newCIO, err := creator(id)
		if newCIO != nil {
			i.lock.Lock()
			defer i.lock.Unlock()
			if _, containerIsTracked := i.ioReaders[containerID]; containerIsTracked {
				i.ioReaders[containerID][taskID] = newCIO
			} else {
				i.ioReaders[containerID] = map[string]cio.IO{
					taskID: newCIO,
				}
			}
		}
		return newCIO, err
	}
}

// Creates and attaches new Readers to the containerd task FIFO files and closes
// any previous Readers that were reading the FIFO files. We must attach our new
// readers to the FIFO files BEFORE closing the previous readers. If we close
// the previous readers first, the FIFO files will be deleted and our new
// readers will fail to attach to the FIFO files.
func (i *ioManager) Attach(containerID, taskID string, attach cio.Attach) cio.Attach {
	return func(f *cio.FIFOSet) (cio.IO, error) {
		i.lock.Lock()
		defer i.lock.Unlock()
		prevIO, exists := i.ioReaders[containerID][taskID]

		newCIO, err := attach(f)
		if newCIO != nil {
			if _, containerIsTracked := i.ioReaders[containerID]; containerIsTracked {
				i.ioReaders[containerID][taskID] = newCIO
			} else {
				i.ioReaders[containerID] = map[string]cio.IO{
					taskID: newCIO,
				}
			}
		}

		if exists && prevIO != nil {
			// Calling Cancel() stops the old cio's from reading the FIFO files
			prevIO.Cancel()
			// Don't call prevIO.Close() because that can result in containerd
			// deleting the FIFO files and re-attachment to fail.
		}

		return newCIO, err
	}
}

func (i *ioManager) Delete(containerId string) {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.ioReaders, containerId)
}

func (i *ioManager) Get(containerID, taskID string) (cio.IO, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()
	cIO, exists := i.ioReaders[containerID][taskID]
	return cIO, exists
}
