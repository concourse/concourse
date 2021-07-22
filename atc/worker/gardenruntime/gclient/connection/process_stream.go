package connection

import (
	"net"
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/transport"
)

type processStream struct {
	processID string
	conn      net.Conn

	sync.Mutex
}

func (s *processStream) Write(data []byte) (int, error) {
	d := string(data)
	stdin := transport.Stdin
	return len(data), s.sendPayload(transport.ProcessPayload{
		ProcessID: s.processID,
		Source:    &stdin,
		Data:      &d,
	})
}

func (s *processStream) Close() error {
	stdin := transport.Stdin
	return s.sendPayload(transport.ProcessPayload{
		ProcessID: s.processID,
		Source:    &stdin,
	})
}

func (s *processStream) SetTTY(spec garden.TTYSpec) error {
	return s.sendPayload(&transport.ProcessPayload{
		ProcessID: s.processID,
		TTY:       &spec,
	})
}

func (s *processStream) Signal(signal garden.Signal) error {
	return s.sendPayload(&transport.ProcessPayload{
		ProcessID: s.processID,
		Signal:    &signal,
	})
}

func (s *processStream) sendPayload(payload interface{}) error {
	s.Lock()

	err := transport.WriteMessage(s.conn, payload)
	if err != nil {
		s.Unlock()
		return err
	}

	s.Unlock()

	return nil
}

func (s *processStream) ProcessID() string {
	return s.processID
}
