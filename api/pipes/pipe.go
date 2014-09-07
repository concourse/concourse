package pipes

import "io"

type Pipe struct {
	ID string `json:"id"`

	PeerAddr string `json:"peer_addr"`

	read  io.ReadCloser  `json:"-"`
	write io.WriteCloser `json:"-"`
}
