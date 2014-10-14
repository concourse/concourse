package logfanout

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrSlowConsumer = errors.New("slow consumer")

type AsyncSink struct {
	msgs chan<- *json.RawMessage
	stop chan<- struct{}
}

func NewAsyncSink(sink Sink, bufferSize int) Sink {
	msgs := make(chan *json.RawMessage, bufferSize)
	stop := make(chan struct{})

	go func() {
		for {
			select {
			case msg := <-msgs:
				err := sink.WriteMessage(msg)
				if err != nil {
					return
				}

			case <-stop:
				sink.Close()
				return
			}
		}
	}()

	return &AsyncSink{
		msgs: msgs,
		stop: stop,
	}
}

func (sink *AsyncSink) WriteMessage(msg *json.RawMessage) error {
	tooSlow := time.NewTimer(10 * time.Second)

	select {
	case sink.msgs <- msg:
		tooSlow.Stop()
		return nil
	case <-tooSlow.C:
		return ErrSlowConsumer
	}
}

func (sink *AsyncSink) Close() error {
	close(sink.stop)
	return nil
}
