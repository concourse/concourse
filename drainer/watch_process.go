package drainer

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . WatchProcess

type WatchProcess interface {
	IsRunning(lager.Logger) (bool, error)
}

type beaconWatchProcess struct {
	pidFile string
}

func NewBeaconWatchProcess(pidFile string) WatchProcess {
	return &beaconWatchProcess{
		pidFile: pidFile,
	}
}

func (p *beaconWatchProcess) IsRunning(logger lager.Logger) (bool, error) {
	_, err := os.Stat(p.pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("beacon-pid-file-does-not-exist", lager.Data{"pidfile": p.pidFile})
			return false, nil
		}

		logger.Error("failed-to-check-if-pid-file-exists", err)
		return false, err
	}

	pidContents, err := ioutil.ReadFile(p.pidFile)
	if err != nil {
		return false, err
	}

	beaconPid := strings.TrimSpace(string(pidContents))

	_, err = os.Stat(fmt.Sprintf("/proc/%s", beaconPid))
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("beacon-process-does-not-exist", lager.Data{"pid": beaconPid})
			return false, nil
		}

		logger.Error("failed-to-check-if-process-exists.", err)
		return false, err
	}

	logger.Debug("beacon-process-is-still-running", lager.Data{"pid": beaconPid})
	return true, nil
}
