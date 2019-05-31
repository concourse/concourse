package resourceserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/radar"
)

//go:generate counterfeiter . ScannerFactory

type ScannerFactory interface {
	NewResourceScanner(pipeline db.Pipeline) radar.Scanner
	NewResourceTypeScanner(dbPipeline db.Pipeline) radar.Scanner
}

type Server struct {
	logger                lager.Logger
	scannerFactory        ScannerFactory
	secretManager         creds.Secrets
	resourceFactory       db.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
}

func NewServer(
	logger lager.Logger,
	scannerFactory ScannerFactory,
	secretManager creds.Secrets,
	resourceFactory db.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
) *Server {
	return &Server{
		logger:                logger,
		scannerFactory:        scannerFactory,
		secretManager:         secretManager,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
	}
}
