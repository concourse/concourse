package resourceserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/radar"
)

//go:generate counterfeiter . ScannerFactory

type ScannerFactory interface {
	NewResourceScanner(pipeline db.Pipeline) radar.Scanner
	NewResourceTypeScanner(dbPipeline db.Pipeline) radar.Scanner
}

type Server struct {
	logger                lager.Logger
	scannerFactory        ScannerFactory
	variablesFactory      creds.VariablesFactory
	resourceFactory       db.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
}

func NewServer(
	logger lager.Logger,
	scannerFactory ScannerFactory,
	variablesFactory creds.VariablesFactory,
	resourceFactory db.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
) *Server {
	return &Server{
		logger:                logger,
		scannerFactory:        scannerFactory,
		variablesFactory:      variablesFactory,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
	}
}
