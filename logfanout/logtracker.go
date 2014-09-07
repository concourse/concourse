package logfanout

import (
	"fmt"
	"io"
	"sync"
)

type Tracker struct {
	db LogDB

	draining    bool
	connections map[io.Closer]struct{}

	logs    map[string]*LogFanout
	parties map[string]int

	lock *sync.RWMutex
}

func NewTracker(db LogDB) *Tracker {
	return &Tracker{
		db: db,

		logs:        make(map[string]*LogFanout),
		parties:     make(map[string]int),
		connections: make(map[io.Closer]struct{}),

		lock: new(sync.RWMutex),
	}
}

func (tracker *Tracker) Register(job string, build string, conn io.Closer) *LogFanout {
	key := fmt.Sprintf("%s-%s", job, build)

	tracker.lock.Lock()

	tracker.connections[conn] = struct{}{}

	logFanout, found := tracker.logs[key]
	if !found {
		logFanout = NewLogFanout(job, build, tracker.db)
		tracker.logs[key] = logFanout
		tracker.parties[key]++
	}

	draining := tracker.draining

	tracker.lock.Unlock()

	if draining {
		conn.Close()
	}

	return logFanout
}

func (tracker *Tracker) Unregister(job string, build string, conn io.Closer) {
	key := fmt.Sprintf("%s-%s", job, build)

	tracker.lock.Lock()

	tracker.parties[key]--

	delete(tracker.connections, conn)

	if tracker.parties[key] == 0 {
		delete(tracker.logs, key)
		delete(tracker.parties, key)
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
