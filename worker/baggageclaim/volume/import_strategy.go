package volume

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/volume/copy"
)

type ImportStrategy struct {
	Path           string
	FollowSymlinks bool
}

func (strategy ImportStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem, streamer Streamer) (FilesystemInitVolume, error) {
	initVolume, err := fs.NewVolume(handle)
	if err != nil {
		return nil, err
	}

	destination := initVolume.DataPath()

	info, err := os.Stat(strategy.Path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		err = copy.Cp(strategy.FollowSymlinks, filepath.Clean(strategy.Path), destination)
		if err != nil {
			return nil, err
		}
	} else {
		tgzFile, err := os.Open(strategy.Path)
		if err != nil {
			return nil, err
		}

		defer tgzFile.Close()

		invalid, err := streamer.In(tgzFile, destination, true)
		if err != nil {
			if invalid {
				logger.Info("malformed-archive", lager.Data{
					"error": err.Error(),
				})
			} else {
				logger.Error("failed-to-stream-in", err)
			}

			return nil, err
		}
	}

	return initVolume, nil
}

func (ImportStrategy) String() string {
	return StrategyImport
}
