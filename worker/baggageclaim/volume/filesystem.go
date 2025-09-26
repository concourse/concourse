package volume

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Filesystem

type Filesystem interface {
	NewVolume(string) (FilesystemInitVolume, error)
	LookupVolume(string) (FilesystemLiveVolume, bool, error)
	LookupVolumeWithSize(string) (FilesystemLiveVolume, bool, int, error)
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

type filesystem struct {
	driver Driver

	initDir string
	liveDir string
	deadDir string
}

func NewFilesystem(driver Driver, parentDir string) (Filesystem, error) {
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
		driver: driver,

		initDir: initDir,
		liveDir: liveDir,
		deadDir: deadDir,
	}, nil
}

func (fs *filesystem) NewVolume(handle string) (FilesystemInitVolume, error) {
	volume, err := fs.initRawVolume(handle)
	if err != nil {
		return nil, err
	}

	err = fs.driver.CreateVolume(volume)
	if err != nil {
		volume.cleanup()
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

func (fs *filesystem) LookupVolumeWithSize(handle string) (FilesystemLiveVolume, bool, int, error) {
	volumePath := fs.liveVolumePath(handle)

	info, err := os.Stat(volumePath)
	if os.IsNotExist(err) {
		return nil, false, 0, nil
	}

	if err != nil {
		return nil, false, 0, err
	}

	if !info.IsDir() {
		return nil, false, 0, nil
	}

	volumeSize, err := getDirSize(volumePath)
	if err != nil {
		return nil, false, 0, err
	}

	return &liveVolume{
		baseVolume: baseVolume{
			fs: fs,

			handle: handle,
			dir:    volumePath,
		},
	}, true, volumeSize, nil
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

func (base *baseVolume) cleanup() error {
	return os.RemoveAll(base.dir)
}

func (base *baseVolume) parentLink() string {
	return filepath.Join(base.dir, "parent")
}

type initVolume struct {
	baseVolume
}

func (vol *initVolume) Initialize() (FilesystemLiveVolume, error) {
	liveDir := vol.fs.liveVolumePath(vol.handle)

	err := os.Rename(vol.dir, liveDir)
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

type liveVolume struct {
	baseVolume
}

func (vol *liveVolume) NewSubvolume(handle string) (FilesystemInitVolume, error) {
	child, err := vol.fs.initRawVolume(handle)
	if err != nil {
		return nil, err
	}

	err = vol.fs.driver.CreateCopyOnWriteLayer(child, vol)
	if err != nil {
		child.cleanup()
		return nil, err
	}

	err = os.Symlink(vol.dir, child.parentLink())
	if err != nil {
		child.Destroy()
		return nil, err
	}

	return child, nil
}

type deadVolume struct {
	baseVolume
}

func (vol *deadVolume) Destroy() error {
	err := vol.fs.driver.DestroyVolume(vol)
	if err != nil {
		return err
	}

	return vol.cleanup()
}

// Run "du -sm $path" command and get directory size in megabytes
// TODO: du command may not exist on worker
// TODO: replace implementation with github.com/ricochet2200/go-disk-usage
func getDirSize(path string) (int, error) {
	cmd := exec.Command("du", "-sm", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(`(\d+)\s+`)
	match := re.FindStringSubmatch(string(out))
	sizeStr := match[1]
	size, _ := strconv.Atoi(sizeStr)
	return size, nil
}
