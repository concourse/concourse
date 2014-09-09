package pipes

import (
	"sync"

	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	peerAddr string

	pipes  map[string]pipe
	pipesL *sync.RWMutex
}

func NewServer(logger lager.Logger, peerAddr string) *Server {
	return &Server{
		logger: logger,

		peerAddr: peerAddr,

		pipes:  make(map[string]pipe),
		pipesL: new(sync.RWMutex),
	}
}
