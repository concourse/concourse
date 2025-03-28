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
	if len(data) == 0 {
		return 0, nil
	}

	d := string(data)
	stdin := transport.Stdin

	err := s.sendPayload(transport.ProcessPayload{
		ProcessID: s.processID,
		Source:    &stdin,
		Data:      &d,
	})

	if err != nil {
		return 0, err
	}

	return len(data), nil
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

func (s *processStream) sendPayload(payload any) error {
	s.Lock()
	defer s.Unlock()

	return transport.WriteMessage(s.conn, payload)
}

func (s *processStream) ProcessID() string {
	return s.processID
}
