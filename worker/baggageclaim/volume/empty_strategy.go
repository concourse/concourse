package volume

import "code.cloudfoundry.org/lager"

type EmptyStrategy struct{}

func (EmptyStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem, streamer Streamer) (FilesystemInitVolume, error) {
	return fs.NewVolume(handle)
}

func (EmptyStrategy) String() string {
	return StrategyEmpty
}
