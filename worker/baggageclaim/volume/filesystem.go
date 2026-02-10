package volume

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Filesystem

type Filesystem interface {
	NewVolume(string) (FilesystemInitVolume, error)
	LookupVolume(string) (FilesystemLiveVolume, bool, error)
	ListVolumes() ([]FilesystemLiveVolume, error)
}

//counterfeiter:generate . FilesystemVolume

// FilesystemVolume represents the state of a volume's data and metadata.
//
// Operations will return ErrVolumeDoesNotExist if the data on disk has
// disappeared.
type FilesystemVolume interface {
	Handle() string

	DataPath() string

	LoadProperties() (Properties, error)
	StoreProperties(Properties) error

	LoadPrivileged() (bool, error)
	StorePrivileged(bool) error

	Parent() (FilesystemLiveVolume, bool, error)

	Destroy() error
}

//counterfeiter:generate . FilesystemInitVolume

type FilesystemInitVolume interface {
	FilesystemVolume

	Initialize() (FilesystemLiveVolume, error)
}

//counterfeiter:generate . FilesystemLiveVolume

type FilesystemLiveVolume interface {
	FilesystemVolume

	NewSubvolume(handle string) (FilesystemInitVolume, error)
}

const (
	initDirname = "init" // volumes being initialized
	liveDirname = "live" // volumes accessible via API
	deadDirname = "dead" // volumes being torn down
)

var _ Filesystem = (*filesystem)(nil)

type filesystem struct {
	log    lager.Logger
	driver Driver

	initDir string
	liveDir string
	deadDir string
}

func NewFilesystem(logger lager.Logger, driver Driver, parentDir string) (Filesystem, error) {
	initDir := filepath.Join(parentDir, initDirname)
	liveDir := filepath.Join(parentDir, liveDirname)
	deadDir := filepath.Join(parentDir, deadDirname)

	err := os.MkdirAll(initDir, 0755)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(liveDir, 0755)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(deadDir, 0755)
	if err != nil {
		return nil, err
	}

	return &filesystem{
		log:    logger.Session("filesystem"),
		driver: driver,

		initDir: initDir,
		liveDir: liveDir,
		deadDir: deadDir,
	}, nil
}

func (fs *filesystem) NewVolume(handle string) (FilesystemInitVolume, error) {
	fs.log.Debug("new-volume", lager.Data{"handle": handle})
	volume, err := fs.initRawVolume(handle)
	if err != nil {
		return nil, err
	}

	err = fs.driver.CreateVolume(volume)
	if err != nil {
		e := volume.Destroy()
		if e != nil {
			fs.log.Error("error-cleaning-up-volume-after-failed-creation", e)
		}
		return nil, err
	}

	return volume, nil
}

func (fs *filesystem) LookupVolume(handle string) (FilesystemLiveVolume, bool, error) {
	volumePath := fs.liveVolumePath(handle)

	info, err := os.Stat(volumePath)
	if os.IsNotExist(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	if !info.IsDir() {
		return nil, false, nil
	}

	return &liveVolume{
		baseVolume: baseVolume{
			fs: fs,

			handle: handle,
			dir:    volumePath,
		},
	}, true, nil
}

func (fs *filesystem) ListVolumes() ([]FilesystemLiveVolume, error) {
	liveDirs, err := os.ReadDir(fs.liveDir)
	if err != nil {
		return nil, err
	}

	response := make([]FilesystemLiveVolume, 0, len(liveDirs))

	for _, liveDir := range liveDirs {
		handle := liveDir.Name()

		response = append(response, &liveVolume{
			baseVolume: baseVolume{
				fs: fs,

				handle: handle,
				dir:    fs.liveVolumePath(handle),
			},
		})
	}

	fs.log.Debug("list-volumes", lager.Data{"live-volumes": len(response)})

	return response, nil
}

func (fs *filesystem) initRawVolume(handle string) (*initVolume, error) {
	volumePath := fs.initVolumePath(handle)

	err := os.Mkdir(volumePath, 0755)
	if err != nil {
		return nil, err
	}

	volume := &initVolume{
		baseVolume: baseVolume{
			fs: fs,

			handle: handle,
			dir:    volumePath,
		},
	}

	err = volume.StoreProperties(Properties{})
	if err != nil {
		return nil, err
	}

	return volume, nil
}

func (fs *filesystem) initVolumePath(handle string) string {
	return filepath.Join(fs.initDir, handle)
}

func (fs *filesystem) liveVolumePath(handle string) string {
	return filepath.Join(fs.liveDir, handle)
}

func (fs *filesystem) deadVolumePath(handle string) string {
	return filepath.Join(fs.deadDir, handle)
}

var _ FilesystemVolume = (*baseVolume)(nil)

type baseVolume struct {
	fs *filesystem

	handle string
	dir    string
}

func (base *baseVolume) Handle() string {
	return base.handle
}

func (base *baseVolume) DataPath() string {
	return filepath.Join(base.dir, "volume")
}

func (base *baseVolume) LoadProperties() (Properties, error) {
	return (&Metadata{base.dir}).Properties()
}

func (base *baseVolume) StoreProperties(newProperties Properties) error {
	return (&Metadata{base.dir}).StoreProperties(newProperties)
}

func (base *baseVolume) LoadPrivileged() (bool, error) {
	return (&Metadata{base.dir}).IsPrivileged()
}

func (base *baseVolume) StorePrivileged(isPrivileged bool) error {
	return (&Metadata{base.dir}).StorePrivileged(isPrivileged)
}

func (base *baseVolume) Parent() (FilesystemLiveVolume, bool, error) {
	parentDir, err := filepath.EvalSymlinks(base.parentLink())
	if os.IsNotExist(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	return &liveVolume{
		baseVolume: baseVolume{
			fs: base.fs,

			handle: filepath.Base(parentDir),
			dir:    parentDir,
		},
	}, true, nil
}

func (base *baseVolume) Destroy() error {
	base.fs.log.Debug("destroy-volume", lager.Data{"handle": base.Handle()})
	deadDir := base.fs.deadVolumePath(base.handle)

	err := os.Rename(base.dir, deadDir)
	if err != nil {
		return err
	}

	deadVol := &deadVolume{
		baseVolume: baseVolume{
			fs: base.fs,

			handle: base.handle,
			dir:    deadDir,
		},
	}

	return deadVol.Destroy()
}

func (base *baseVolume) parentLink() string {
	return filepath.Join(base.dir, "parent")
}

var _ FilesystemInitVolume = (*initVolume)(nil)

type initVolume struct {
	baseVolume
}

func (vol *initVolume) Initialize() (FilesystemLiveVolume, error) {
	vol.fs.log.Debug("init-volume", lager.Data{"handle": vol.Handle()})
	liveDir := vol.fs.liveVolumePath(vol.handle)

	var err error
	for range 5 {
		err = os.Rename(vol.dir, liveDir)
		if err != nil &&
			// Windows specific error that some users have reported seeing.
			// Injecting a sleep resolves the issue. Clearly some kind of
			// race-condition issue, but it's unclear what we're racing with.
			strings.Contains(err.Error(), "Access is denied") {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}

	return &liveVolume{
		baseVolume: baseVolume{
			fs: vol.fs,

			handle: vol.handle,
			dir:    liveDir,
		},
	}, nil
}

var _ FilesystemLiveVolume = (*liveVolume)(nil)

type liveVolume struct {
	baseVolume
}

func (vol *liveVolume) NewSubvolume(handle string) (FilesystemInitVolume, error) {
	vol.fs.log.Debug("new-subvolume", lager.Data{
		"parent": vol.Handle(),
		"child":  handle,
	})
	child, err := vol.fs.initRawVolume(handle)
	if err != nil {
		return nil, err
	}

	err = vol.fs.driver.CreateCopyOnWriteLayer(child, vol)
	if err != nil {
		e := child.Destroy()
		if e != nil {
			vol.fs.log.Error("error-cleaning-up-volume-after-fail-cow-creation", e)
		}
		return nil, err
	}

	err = os.Symlink(vol.dir, child.parentLink())
	if err != nil {
		e := child.Destroy()
		if e != nil {
			vol.fs.log.Error("error-cleaning-up-volume-after-symlink-call", e)
		}
		return nil, err
	}

	return child, nil
}

var _ FilesystemVolume = (*deadVolume)(nil)

type deadVolume struct {
	baseVolume
}

func (vol *deadVolume) Destroy() error {
	err := vol.fs.driver.DestroyVolume(vol)
	if err != nil {
		vol.fs.log.Error("driver-destroy-volume", err, lager.Data{"handle": vol.Handle()})
	}
	return os.RemoveAll(vol.dir)
}
