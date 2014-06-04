package logfanout

import (
	"errors"
	"io"
	"sync"
)

type LogFanout struct {
	lock *sync.Mutex

	sinks []io.WriteCloser

	closed        bool
	waitForClosed chan struct{}
}

func NewLogFanout() *LogFanout {
	return &LogFanout{
		lock:          new(sync.Mutex),
		waitForClosed: make(chan struct{}),
	}
}

func (log *LogFanout) Write(data []byte) (int, error) {
	log.lock.Lock()

	newSinks := []io.WriteCloser{}
	for _, sink := range log.sinks {
		_, err := sink.Write(data)
		if err != nil {
			continue
		}

		newSinks = append(newSinks, sink)
	}

	log.sinks = newSinks

	log.lock.Unlock()

	return len(data), nil
}

func (log *LogFanout) Attach(sink io.WriteCloser) {
	log.lock.Lock()

	if log.closed {
		sink.Close()
	} else {
		log.sinks = append(log.sinks, sink)
	}

	log.lock.Unlock()

	<-log.waitForClosed
}

func (log *LogFanout) Close() error {
	log.lock.Lock()
	defer log.lock.Unlock()

	if log.closed {
		return errors.New("close twice")
	}

	for _, sink := range log.sinks {
		sink.Close()
	}

	log.closed = true
	log.sinks = nil

	close(log.waitForClosed)

	return nil
}
