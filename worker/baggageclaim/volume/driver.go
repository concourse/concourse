package volume

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Driver
type Driver interface {
	CreateVolume(FilesystemInitVolume) error
	DestroyVolume(FilesystemVolume) error

	CreateCopyOnWriteLayer(FilesystemInitVolume, FilesystemLiveVolume) error

	Recover(Filesystem) error
}
