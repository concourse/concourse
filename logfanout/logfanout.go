package logfanout

import (
	"errors"
	"io"
	"sync"

	"github.com/concourse/atc/db"
)

type LogFanout struct {
	job   string
	build int
	db    db.DB

	lock *sync.Mutex

	sinks []io.WriteCloser

	closed        bool
	waitForClosed chan struct{}
}

func NewLogFanout(job string, build int, db db.DB) *LogFanout {
	return &LogFanout{
		job:   job,
		build: build,
		db:    db,

		lock:          new(sync.Mutex),
		waitForClosed: make(chan struct{}),
	}
}

func (fanout *LogFanout) Write(data []byte) (int, error) {
	fanout.lock.Lock()

	err := fanout.db.AppendBuildLog(fanout.job, fanout.build, data)
	if err != nil {
		return 0, err
	}

	newSinks := []io.WriteCloser{}
	for _, sink := range fanout.sinks {
		_, err := sink.Write(data)
		if err != nil {
			continue
		}

		newSinks = append(newSinks, sink)
	}

	fanout.sinks = newSinks

	fanout.lock.Unlock()

	return len(data), nil
}

func (fanout *LogFanout) Attach(sink io.WriteCloser) error {
	fanout.lock.Lock()

	log, err := fanout.db.BuildLog(fanout.job, fanout.build)
	if err == nil {
		_, err = sink.Write(log)
		if err != nil {
			fanout.lock.Unlock()
			return err
		}
	}

	if fanout.closed {
		sink.Close()
	} else {
		fanout.sinks = append(fanout.sinks, sink)
	}

	fanout.lock.Unlock()

	<-fanout.waitForClosed

	return nil
}

func (fanout *LogFanout) Close() error {
	fanout.lock.Lock()
	defer fanout.lock.Unlock()

	if fanout.closed {
		return errors.New("close twice")
	}

	for _, sink := range fanout.sinks {
		sink.Close()
	}

	fanout.closed = true
	fanout.sinks = nil

	close(fanout.waitForClosed)

	return nil
}
