package volume

import "code.cloudfoundry.org/lager/v3"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Strategy

type Strategy interface {
	Materialize(lager.Logger, string, Filesystem, Streamer) (FilesystemInitVolume, error)
	String() string
}
