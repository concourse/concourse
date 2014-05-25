package drainer

import (
	"io"
	"sync"
)

type Drainer struct {
	connections map[io.Closer]bool
	closing     bool

	mutex *sync.Mutex
}

func NewDrainer() *Drainer {
	return &Drainer{
		connections: make(map[io.Closer]bool),

		mutex: new(sync.Mutex),
	}
}

func (drainer *Drainer) Drain() {
	drainer.mutex.Lock()
	drainer.closing = true
	conns := drainer.connections
	drainer.mutex.Unlock()

	for conn, _ := range conns {
		conn.Close()
	}
}

func (drainer *Drainer) Add(closer io.Closer) {
	drainer.mutex.Lock()

	if drainer.closing {
		drainer.mutex.Unlock()
		closer.Close()
	} else {
		drainer.connections[closer] = true
		drainer.mutex.Unlock()
	}
}

func (drainer *Drainer) Remove(closer io.Closer) {
	drainer.mutex.Lock()
	delete(drainer.connections, closer)
	drainer.mutex.Unlock()
}
