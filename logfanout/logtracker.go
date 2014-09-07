package logfanout

import (
	"io"
	"sync"
)

type Tracker struct {
	db LogDB

	draining    bool
	connections map[io.Closer]struct{}

	logs    map[int]*LogFanout
	parties map[int]int

	lock *sync.RWMutex
}

func NewTracker(db LogDB) *Tracker {
	return &Tracker{
		db: db,

		logs:        make(map[int]*LogFanout),
		parties:     make(map[int]int),
		connections: make(map[io.Closer]struct{}),

		lock: new(sync.RWMutex),
	}
}

func (tracker *Tracker) Register(build int, conn io.Closer) *LogFanout {
	tracker.lock.Lock()

	tracker.connections[conn] = struct{}{}

	logFanout, found := tracker.logs[build]
	if !found {
		logFanout = NewLogFanout(build, tracker.db)
		tracker.logs[build] = logFanout
		tracker.parties[build]++
	}

	draining := tracker.draining

	tracker.lock.Unlock()

	if draining {
		conn.Close()
	}

	return logFanout
}

func (tracker *Tracker) Unregister(build int, conn io.Closer) {
	tracker.lock.Lock()

	tracker.parties[build]--

	delete(tracker.connections, conn)

	if tracker.parties[build] == 0 {
		delete(tracker.logs, build)
		delete(tracker.parties, build)
	}

	tracker.lock.Unlock()
}

func (tracker *Tracker) Drain() {
	tracker.lock.Lock()

	tracker.draining = true

	logs := tracker.logs
	conns := tracker.connections

	tracker.lock.Unlock()

	for _, fanout := range logs {
		fanout.Close()
	}

	for conn, _ := range conns {
		conn.Close()
	}
}
