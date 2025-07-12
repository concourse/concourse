//go:build linux

package runtime

import (
	"sync"

	"github.com/containerd/containerd/v2/pkg/cio"
)

//counterfeiter:generate github.com/containerd/containerd/v2/pkg/cio.IO
//counterfeiter:generate . IOManager

type IOManager interface {
	Creator(procId string, creator cio.Creator) cio.Creator
	Attach(id string, attach cio.Attach) cio.Attach
	Delete(id string)
	Get(id string) (cio.IO, bool)
}

// IOManager keeps track of the Readers that are attached to containerd Task
// FIFO files. IOManager allows us to guarantee that no more than one reader is
// attached to the FIFO files. If multiple readers are attached to the FIFO
// files, the stdout/stderr from a task will get split between all attached
// readers, resulting in us losing logs. The containerd docs state that the
// client (that's us!) have to ensure only one reader is attached to the task
// FIFO files.
type ioManager struct {
	ioReaders map[string]cio.IO
	lock      sync.Mutex
}

func NewIOManager() IOManager {
	return &ioManager{
		ioReaders: map[string]cio.IO{},
		lock:      sync.Mutex{},
	}
}

func (i *ioManager) Creator(procId string, creator cio.Creator) cio.Creator {
	return func(id string) (cio.IO, error) {
		cio, err := creator(id)
		if cio != nil {
			i.lock.Lock()
			defer i.lock.Unlock()
			i.ioReaders[procId] = cio
		}
		return cio, err
	}
}

// Creates and attaches new Readers to the containerd task FIFO files and closes
// any previous Readers that were reading the FIFO files. We must attach our new
// readers to the FIFO files BEFORE closing the previous readers. If we close
// the previous readers first, the FIFO files will be deleted and our new
// readers will fail to attach to the FIFO files.
func (i *ioManager) Attach(id string, attach cio.Attach) cio.Attach {
	return func(f *cio.FIFOSet) (cio.IO, error) {
		i.lock.Lock()
		defer i.lock.Unlock()
		prevIO, exists := i.ioReaders[id]

		cio, err := attach(f)
		if cio != nil {
			i.ioReaders[id] = cio
		}

		if exists && prevIO != nil {
			prevIO.Cancel()
			prevIO.Close()
		}

		return cio, err
	}
}

func (i *ioManager) Delete(id string) {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.ioReaders, id)
}

func (i *ioManager) Get(id string) (cio.IO, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()
	cIO, exists := i.ioReaders[id]
	return cIO, exists
}
