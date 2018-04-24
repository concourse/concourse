package resourceserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/radar"
)

//go:generate counterfeiter . ScannerFactory

type ScannerFactory interface {
	NewResourceScanner(pipeline db.Pipeline) radar.Scanner
}

type Server struct {
	logger           lager.Logger
	scannerFactory   ScannerFactory
	variablesFactory creds.VariablesFactory
}

func NewServer(
	logger lager.Logger,
	scannerFactory ScannerFactory,
	variablesFactory creds.VariablesFactory,
) *Server {
	return &Server{
		logger:           logger,
		scannerFactory:   scannerFactory,
		variablesFactory: variablesFactory,
	}
}
