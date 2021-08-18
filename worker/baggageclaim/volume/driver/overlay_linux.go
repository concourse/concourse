package driver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/copy"
)

var mountOpts string

func init() {
	if metacopySupported() {
		mountOpts = "lowerdir=%s,upperdir=%s,workdir=%s,metacopy=on"
	} else {
		mountOpts = "lowerdir=%s,upperdir=%s,workdir=%s"
	}
}

// Metacopy is an overlayfs feature. If all you're doing is chown/chmod'ing a
// file then it will not create a copy of the file. Files will only be copied
// when they are written to.
func metacopySupported() bool {
	_, err := os.Stat("/sys/module/overlay/parameters/metacopy")
	if err != nil {
		return !errors.Is(err, os.ErrNotExist)
	}
	return true
}

type OverlayDriver struct {
	OverlaysDir string
}

func NewOverlayDriver(overlaysDir string) volume.Driver {
	return &OverlayDriver{
		OverlaysDir: overlaysDir,
	}
}

func (driver *OverlayDriver) CreateVolume(vol volume.FilesystemInitVolume) error {
	path := vol.DataPath()
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}

	return driver.bindMount(vol)
}

func (driver *OverlayDriver) DestroyVolume(vol volume.FilesystemVolume) error {
	path := vol.DataPath()

	err := syscall.Unmount(path, 0)
	// when a path is already unmounted, and unmount is called
	// on it, syscall.EINVAL is returned as an error
	// ignore this error and continue to clean up
	if err != nil && err != os.ErrInvalid {
		return err
	}

	err = os.RemoveAll(driver.workDir(vol))
	if err != nil {
		return err
	}

	err = os.RemoveAll(driver.layerDir(vol))
	if err != nil {
		return err
	}

	return os.RemoveAll(path)
}

func (driver *OverlayDriver) CreateCopyOnWriteLayer(
	child volume.FilesystemInitVolume,
	parent volume.FilesystemLiveVolume,
) error {
	path := child.DataPath()
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	rootParent, err := driver.findRootParent(child, parent)
	if err != nil {
		return err
	}

	return driver.overlayMount(child, rootParent)
}

func (driver *OverlayDriver) Recover(fs volume.Filesystem) error {
	vols, err := fs.ListVolumes()
	if err != nil {
		return err
	}

	type cow struct {
		parent volume.FilesystemLiveVolume
		child  volume.FilesystemLiveVolume
	}

	cows := []cow{}
	for _, vol := range vols {
		parentVol, hasParent, err := vol.Parent()
		if err != nil {
			return fmt.Errorf("get parent: %w", err)
		}

		if hasParent {
			cows = append(cows, cow{
				parent: parentVol,
				child:  vol,
			})
			continue
		}

		err = driver.bindMount(vol)
		if err != nil {
			return fmt.Errorf("recover bind mount: %w", err)
		}
	}

	for _, cow := range cows {
		rootParent, err := driver.findRootParent(cow.child, cow.parent)
		if err != nil {
			return err
		}

		err = driver.overlayMount(cow.child, rootParent)
		if err != nil {
			return fmt.Errorf("recover overlay mount: %w", err)
		}
	}

	return nil
}

func (driver *OverlayDriver) findRootParent(child volume.FilesystemVolume,
	parent volume.FilesystemLiveVolume) (volume.FilesystemLiveVolume, error) {
	rootParent := parent
	grandparent, hasGrandparent, err := parent.Parent()
	if err != nil {
		return nil, err
	}

	if hasGrandparent {
		childDir := driver.layerDir(child)
		parentDir := driver.layerDir(parent)
		err := copy.Cp(false, parentDir, childDir)
		if err != nil {
			return nil, fmt.Errorf("copy parent data to child: %w", err)
		}

		rootParent = grandparent

		// resolve to root volume
		for {
			grandparent, hasGrandparent, err := rootParent.Parent()
			if err != nil {
				return nil, err
			}

			if !hasGrandparent {
				break
			}

			rootParent = grandparent
		}
	}

	return rootParent, nil
}

func (driver *OverlayDriver) bindMount(vol volume.FilesystemVolume) error {
	layerDir := driver.layerDir(vol)
	err := os.MkdirAll(layerDir, 0755)
	if err != nil {
		return err
	}

	err = syscall.Mount(layerDir, vol.DataPath(), "", syscall.MS_BIND, "")
	if err != nil {
		return err
	}

	return nil
}

func (driver *OverlayDriver) overlayMount(child volume.FilesystemVolume, parent volume.FilesystemLiveVolume) error {
	childDir := driver.layerDir(child)
	err := os.MkdirAll(childDir, 0755)
	if err != nil {
		return err
	}

	workDir := driver.workDir(child)
	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		return err
	}

	opts := fmt.Sprintf(
		mountOpts,
		parent.DataPath(), //lowerdir
		childDir,          //upperdir
		workDir,           //workdir
	)

	err = syscall.Mount("overlay", child.DataPath(), "overlay", 0, opts)
	if err != nil {
		return err
	}

	return nil
}

func (driver *OverlayDriver) layerDir(vol volume.FilesystemVolume) string {
	return filepath.Join(driver.OverlaysDir, vol.Handle())
}

func (driver *OverlayDriver) workDir(vol volume.FilesystemVolume) string {
	return filepath.Join(driver.OverlaysDir, "work", vol.Handle())
}
