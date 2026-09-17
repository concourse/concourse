package driver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/copy"
	"github.com/moby/sys/mountinfo"
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

var _ volume.Driver = (*OverlayDriver)(nil)

type OverlayDriver struct {
	OverlaysDir string
	logger      lager.Logger
}

func NewOverlayDriver(logger lager.Logger, overlaysDir string) volume.Driver {
	l := logger.Session("overlay-driver")
	l.Info("metacopy-support", lager.Data{"supported": metacopySupported()})
	return &OverlayDriver{
		OverlaysDir: overlaysDir,
		logger:      l,
	}
}

func (driver *OverlayDriver) CreateVolume(vol volume.FilesystemInitVolume) error {
	path := vol.DataPath()
	driver.logger.Debug("create-volume", lager.Data{"path": path})

	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}

	_, err = driver.bindMount(vol)
	return err
}

func (driver *OverlayDriver) DestroyVolume(vol volume.FilesystemVolume) error {
	path := vol.DataPath()
	driver.logger.Debug("destroy-volume", lager.Data{"path": path})

	err := syscall.Unmount(path, 0)
	// when a path is already unmounted, and unmount is called
	// on it, syscall.EINVAL is returned as an error
	// ignore this error and continue to clean up
	if err != nil && !errors.Is(err, syscall.EINVAL) {
		driver.logger.Error("unmount", err, lager.Data{"path": path})
	}

	workDir := driver.workDir((vol))
	err = os.RemoveAll(workDir)
	if err != nil {
		driver.logger.Error("rm-work-dir", err, lager.Data{"path": workDir})
	}

	layerDir := driver.layerDir(vol)
	err = os.RemoveAll(layerDir)
	if err != nil {
		driver.logger.Error("rm-layer-dir", err, lager.Data{"path": layerDir})
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

	driver.logger.Debug("create-cow", lager.Data{
		"child-path":  path,
		"parent-path": rootParent.DataPath(),
	})

	_, err = driver.overlayMount(child, rootParent)
	return err
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

	// tracks paths mounted during this Recover call so they
	// can be rolled back if a later step fails.
	var mountedThisPass []string
	rollback := func() {
		for _, path := range mountedThisPass {
			if err := syscall.Unmount(path, 0); err != nil && !errors.Is(err, syscall.EINVAL) {
				driver.logger.Error("rollback-unmount", err, lager.Data{"path": path})
			}
		}
	}

	cows := []cow{}
	for _, vol := range vols {
		parentVol, hasParent, err := vol.Parent()
		if err != nil {
			rollback()
			return fmt.Errorf("get parent: %w", err)
		}

		if hasParent {
			cows = append(cows, cow{
				parent: parentVol,
				child:  vol,
			})
			continue
		}

		wasAlreadyMounted, err := driver.bindMount(vol)
		if err != nil {
			rollback()
			return fmt.Errorf("recover bind mount: %w", err)
		}
		if !wasAlreadyMounted {
			mountedThisPass = append(mountedThisPass, vol.DataPath())
		}
	}

	driver.logger.Debug("recovering-mounts", lager.Data{"volumes-to-recover": len(cows)})

	for _, cow := range cows {
		rootParent, err := driver.findRootParent(cow.child, cow.parent)
		if err != nil {
			rollback()
			return err
		}

		wasAlreadyMounted, err := driver.overlayMount(cow.child, rootParent)
		if err != nil {
			rollback()
			return fmt.Errorf("recover overlay mount: %w", err)
		}
		if !wasAlreadyMounted {
			mountedThisPass = append(mountedThisPass, cow.child.DataPath())
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

func (driver *OverlayDriver) bindMount(vol volume.FilesystemVolume) (bool, error) {
	layerDir := driver.layerDir(vol)
	err := os.MkdirAll(layerDir, 0755)
	if err != nil {
		return false, err
	}

	mountPath := vol.DataPath()

	if mounted, err := mountinfo.Mounted(mountPath); err != nil {
		return false, fmt.Errorf("check bind mount: %w", err)
	} else if mounted {
		driver.logger.Debug("bind-mount-already-present", lager.Data{"mount-path": mountPath})
		return true, nil
	}

	driver.logger.Debug("creating-bind-mount", lager.Data{
		"layer-path": layerDir,
		"mount-path": mountPath,
	})

	err = syscall.Mount(layerDir, mountPath, "", syscall.MS_BIND, "")
	if err != nil {
		return false, err
	}

	return false, nil
}

func (driver *OverlayDriver) overlayMount(child volume.FilesystemVolume, parent volume.FilesystemLiveVolume) (bool, error) {
	childDir := driver.layerDir(child)
	err := os.MkdirAll(childDir, 0755)
	if err != nil {
		return false, err
	}

	workDir := driver.workDir(child)
	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		return false, err
	}

	mountPath := child.DataPath()

	if mounted, err := mountinfo.Mounted(mountPath); err != nil {
		return false, fmt.Errorf("check overlay mount: %w", err)
	} else if mounted {
		driver.logger.Debug("overlay-mount-already-present", lager.Data{"mount-path": mountPath})
		return true, nil
	}

	opts := fmt.Sprintf(
		mountOpts,
		parent.DataPath(), //lowerdir
		childDir,          //upperdir
		workDir,           //workdir
	)

	driver.logger.Debug("creating-overlay-mount", lager.Data{"opts": opts})
	err = syscall.Mount("overlay", child.DataPath(), "overlay", 0, opts)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (driver *OverlayDriver) layerDir(vol volume.FilesystemVolume) string {
	return filepath.Join(driver.OverlaysDir, vol.Handle())
}

func (driver *OverlayDriver) workDir(vol volume.FilesystemVolume) string {
	return filepath.Join(driver.OverlaysDir, "work", vol.Handle())
}

func (driver *OverlayDriver) RemoveOrphanedResources(isKnown func(handle string) bool) error {
	if isKnown == nil {
		return errors.New("must pass in a function")
	}

	_, err := os.Stat(driver.OverlaysDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// If dir does not exist yet then there's nothing to remove
			return nil
		} else {
			return fmt.Errorf("checking overlay dir exists: %w", err)
		}
	}

	entries, err := os.ReadDir(driver.OverlaysDir)
	if err != nil {
		return fmt.Errorf("read overlays dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()

		// skip the "work" meta-directory itself. Its children are handled below
		if name == "work" {
			continue
		}

		if isKnown(name) {
			continue
		}

		driver.logger.Debug("removing-orphaned-overlay-layer", lager.Data{"handle": name})

		layerPath := filepath.Join(driver.OverlaysDir, name)
		err := os.RemoveAll(layerPath)
		if err != nil {
			driver.logger.Error("failed-to-remove-orphaned-layer", err, lager.Data{"handle": name})
		}

		workPath := filepath.Join(driver.OverlaysDir, "work", name)
		err = os.RemoveAll(workPath)
		if err != nil {
			driver.logger.Error("failed-to-remove-orphaned-work-dir", err, lager.Data{"handle": name})
		}
	}

	// scan work/ dir for orphaned entries that may exist without a layer dir
	workDir := filepath.Join(driver.OverlaysDir, "work")
	workEntries, err := os.ReadDir(workDir)
	if err != nil {
		return fmt.Errorf("read overlays work dir: %w", err)
	}

	for _, entry := range workEntries {
		name := entry.Name()
		if isKnown(name) {
			continue
		}

		driver.logger.Debug("removing-orphaned-overlay-work-dir", lager.Data{"handle": name})

		workPath := filepath.Join(workDir, name)
		if err := os.RemoveAll(workPath); err != nil {
			driver.logger.Error("failed-to-remove-orphaned-work-dir", err, lager.Data{"handle": name})
		}
	}

	return nil
}
