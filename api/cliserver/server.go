package cliserver

import "github.com/pivotal-golang/lager"

type Server struct {
	logger          lager.Logger
	cliDownloadsDir string
}

func NewServer(logger lager.Logger, cliDownloadsDir string) *Server {
	return &Server{
		logger:          logger,
		cliDownloadsDir: cliDownloadsDir,
	}
}
