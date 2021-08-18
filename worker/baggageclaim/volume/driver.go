package volume

//go:generate counterfeiter . Driver

type Driver interface {
	CreateVolume(FilesystemInitVolume) error
	DestroyVolume(FilesystemVolume) error

	CreateCopyOnWriteLayer(FilesystemInitVolume, FilesystemLiveVolume) error

	Recover(Filesystem) error
}
