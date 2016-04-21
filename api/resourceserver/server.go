package resourceserver

import (
	"github.com/concourse/atc/radar"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . ScannerFactory

type ScannerFactory interface {
	NewResourceScanner(db radar.RadarDB) radar.Scanner
}

type Server struct {
	logger         lager.Logger
	scannerFactory ScannerFactory
}

func NewServer(logger lager.Logger, scannerFactory ScannerFactory) *Server {
	return &Server{
		logger:         logger,
		scannerFactory: scannerFactory,
	}
}
