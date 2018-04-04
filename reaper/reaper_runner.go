package reaper

import (
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

func NewReaper(gardenAddr string, port string, logger lager.Logger) *ReaperCmd {
	return &ReaperCmd{
		Logger:     logger,
		GardenAddr: gardenAddr,
		Port:       port,
	}
}

func NewReaperRunner(logger lager.Logger, gardenAddr string, port string) ifrit.Runner {
	logger = logger.Session("reaper-server")
	reaperR := NewReaper(gardenAddr, port, logger)
	return ifrit.RunFunc(reaperR.Run)
}
