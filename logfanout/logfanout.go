package logfanout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type LogDB interface {
	BuildLog(job string, build string) ([]byte, error)
	AppendBuildLog(job string, build string, log []byte) error
}

type LogFanout struct {
	job   string
	build string
	db    LogDB

	lock *sync.Mutex

	sinks []*websocket.Conn

	closed        bool
	waitForClosed chan struct{}
}

func NewLogFanout(job string, build string, db LogDB) *LogFanout {
	return &LogFanout{
		job:   job,
		build: build,
		db:    db,

		lock:          new(sync.Mutex),
		waitForClosed: make(chan struct{}),
	}
}

func (fanout *LogFanout) WriteMessage(msg *json.RawMessage) error {
	fanout.lock.Lock()
	defer fanout.lock.Unlock()

	err := fanout.db.AppendBuildLog(fanout.job, fanout.build, []byte(*msg))
	if err != nil {
		return err
	}

	newSinks := []*websocket.Conn{}
	for _, sink := range fanout.sinks {
		err := sink.WriteJSON(msg)
		if err != nil {
			continue
		}

		newSinks = append(newSinks, sink)
	}

	fanout.sinks = newSinks

	return nil
}

func (fanout *LogFanout) Attach(sink *websocket.Conn) error {
	fanout.lock.Lock()

	buildLog, err := fanout.db.BuildLog(fanout.job, fanout.build)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewBuffer(buildLog))

		for {
			var msg *json.RawMessage
			err := decoder.Decode(&msg)
			if err != nil {
				if err != io.EOF {
					fanout.emitBackwardsCompatible(sink, buildLog)
				}

				break
			}

			err = sink.WriteJSON(msg)
			if err != nil {
				fanout.lock.Unlock()
				return err
			}
		}
	}

	if fanout.closed {
		sink.Close()
	} else {
		fanout.sinks = append(fanout.sinks, sink)
	}

	fanout.lock.Unlock()

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

func (fanout *LogFanout) emitBackwardsCompatible(sink *websocket.Conn, log []byte) {
	err := sink.WriteMessage(websocket.TextMessage, []byte(`{"version":"0.0"}`))
	if err != nil {
		return
	}

	var dangling []byte
	for i := 0; i < len(log); i += 1024 {
		end := i + 1024
		if end > len(log) {
			end = len(log)
		}

		text := append(dangling, log[i:end]...)

		checkEncoding, _ := utf8.DecodeLastRune(text)
		if checkEncoding == utf8.RuneError {
			dangling = text
			continue
		}

		err := sink.WriteMessage(websocket.TextMessage, text)
		if err != nil {
			return
		}

		dangling = nil
	}
}
