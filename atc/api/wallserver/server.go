package wallserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	wall   db.Wall
	logger lager.Logger
}

func NewServer(wall db.Wall, logger lager.Logger) *Server {
	return &Server{
		wall:   wall,
		logger: logger,
	}
}
