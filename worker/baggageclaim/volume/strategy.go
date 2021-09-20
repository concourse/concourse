package volume

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . Strategy

type Strategy interface {
	Materialize(lager.Logger, string, Filesystem, Streamer) (FilesystemInitVolume, error)
	String() string
}
