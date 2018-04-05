package reaper

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/restart"
	"golang.org/x/crypto/ssh"
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

	return restart.Restarter{
		Runner: ifrit.RunFunc(reaperR.Run),
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			if prevErr == nil {
				return nil
			}

			if _, ok := prevErr.(*ssh.ExitError); !ok {
				logger.Error("restarting", prevErr)
				time.Sleep(5 * time.Second)
				return ifrit.RunFunc(reaperR.Run)
			}
			return nil
		},
	}
}
