//go:build windows

package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// prepareContainer converts the container's rootfs into Windows container
// layers and attaches the container to the worker's network.
func (b *GardenBackend) prepareContainer(oci *specs.Spec, handle string) error {
	err := b.prepareRootfs(oci)
	if err != nil {
		return fmt.Errorf("prepare rootfs: %w", err)
	}

	if attacher, ok := b.network.(ContainerAttacher); ok {
		err = attacher.AttachContainer(handle, oci)
		if err != nil {
			return fmt.Errorf("attach container network: %w", err)
		}
	}

	return nil
}

// prepareRootfs converts the container's rootfs directory into a Windows
// container base layer and creates the scratch layer that the runhcs shim
// requires, mirroring what containerd's Windows snapshotter does when
// unpacking an image. The runtime mounts the layers itself, so the spec's
// Root is cleared in favour of Windows.LayerFolders.
func (b *GardenBackend) prepareRootfs(oci *specs.Spec) error {
	if oci.Root == nil || oci.Root.Path == "" {
		return fmt.Errorf("rootfs not set in spec")
	}

	rootfs := oci.Root.Path
	volumeDir := filepath.Dir(rootfs)

	// hcsshim expects the layer's content under Files/. Images extracted
	// from OCI Windows layers already have that structure; plain directory
	// trees get moved underneath it.
	layerDir := rootfs
	if _, err := os.Stat(filepath.Join(rootfs, "Files")); os.IsNotExist(err) {
		layerDir = filepath.Join(volumeDir, "layer")
		if _, err := os.Stat(layerDir); os.IsNotExist(err) {
			err = os.Mkdir(layerDir, 0755)
			if err != nil {
				return fmt.Errorf("mkdir layer dir: %w", err)
			}

			err = os.Rename(rootfs, filepath.Join(layerDir, "Files"))
			if err != nil {
				return fmt.Errorf("move rootfs into layer: %w", err)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(layerDir, "blank.vhdx")); os.IsNotExist(err) {
		err = mergeDeltaHives(layerDir)
		if err != nil {
			return fmt.Errorf("merge delta hives: %w", err)
		}

		// Flattened multi-layer images carry per-layer artifacts that the
		// base layer processing wants to create itself (Hives) or that only
		// hyper-v isolation would use (UtilityVM); processing fails if
		// they're present.
		for _, artifact := range []string{"UtilityVM", "Hives"} {
			err = os.RemoveAll(filepath.Join(layerDir, artifact))
			if err != nil {
				return fmt.Errorf("remove layer artifact %s: %w", artifact, err)
			}
		}

		err = scrubLayerState(layerDir)
		if err != nil {
			return fmt.Errorf("scrub layer state: %w", err)
		}

		// ConvertToBaseLayer creates minimal registry hives but fails on
		// images that already ship them; process those directly.
		systemHive := filepath.Join(layerDir, "Files", "Windows", "System32", "config", "SYSTEM")
		if _, err := os.Stat(systemHive); err == nil {
			err = hcsshim.ProcessBaseLayer(layerDir)
			if err != nil {
				return fmt.Errorf("process base layer: %w", err)
			}
		} else {
			err = hcsshim.ConvertToBaseLayer(layerDir)
			if err != nil {
				return fmt.Errorf("convert to base layer: %w", err)
			}
		}
	}

	scratchDir := filepath.Join(volumeDir, "scratch")
	sandbox := filepath.Join(scratchDir, "sandbox.vhdx")
	if _, err := os.Stat(sandbox); os.IsNotExist(err) {
		err = os.MkdirAll(scratchDir, 0755)
		if err != nil {
			return fmt.Errorf("mkdir scratch dir: %w", err)
		}

		err = copyFile(filepath.Join(layerDir, "blank.vhdx"), sandbox)
		if err != nil {
			return fmt.Errorf("create sandbox vhdx: %w", err)
		}
	}

	oci.Root = nil
	oci.Windows.LayerFolders = []string{layerDir, scratchDir}

	return nil
}

// mergeDeltaHives merges the registry differencing hives of the image's last
// layer onto the base registry. Only the last layer's deltas survive image
// flattening, but for the common servercore case - a base layer plus one
// cumulative update layer - that is exactly the delta that has to be applied
// so the updated system files agree with the registry (e.g. driver config).
func mergeDeltaHives(layerDir string) error {
	deltas := map[string]string{
		"System_Delta":      filepath.Join("Windows", "System32", "config", "SYSTEM"),
		"Software_Delta":    filepath.Join("Windows", "System32", "config", "SOFTWARE"),
		"Sam_Delta":         filepath.Join("Windows", "System32", "config", "SAM"),
		"Security_Delta":    filepath.Join("Windows", "System32", "config", "SECURITY"),
		"DefaultUser_Delta": filepath.Join("Users", "Default", "NTUSER.DAT"),
	}

	for delta, baseRel := range deltas {
		deltaPath := filepath.Join(layerDir, "Hives", delta)
		if _, err := os.Stat(deltaPath); os.IsNotExist(err) {
			continue
		}

		basePath := filepath.Join(layerDir, "Files", baseRel)
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		err := mergeHive(basePath, deltaPath)
		if err != nil {
			return fmt.Errorf("merge %s: %w", delta, err)
		}
	}

	return nil
}

// scrubLayerState removes machine state that the containers used to build
// the image's layers left behind: user profile hives, DPAPI master keys,
// event logs and other runtime files. The registry deltas this state belongs
// with don't survive image flattening, and booting with the mismatched state
// makes the container exit during OS startup.
func scrubLayerState(layerDir string) error {
	stateArtifacts := []string{
		filepath.Join("Users", "ContainerAdministrator"),
		filepath.Join("Users", "ContainerUser"),
		filepath.Join("Windows", "System32", "LogFiles"),
		filepath.Join("Windows", "System32", "winevt", "Logs"),
		filepath.Join("Windows", "System32", "Microsoft", "Protect", "S-1-5-18", "User"),
		filepath.Join("Windows", "ServiceState"),
	}

	for _, artifact := range stateArtifacts {
		err := os.RemoveAll(filepath.Join(layerDir, "Files", artifact))
		if err != nil {
			return fmt.Errorf("remove %s: %w", artifact, err)
		}
	}

	// service account profile hives from the build containers
	profilesDir := filepath.Join(layerDir, "Files", "Windows", "ServiceProfiles")
	profiles, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read service profiles: %w", err)
	}

	for _, profile := range profiles {
		if !profile.IsDir() {
			continue
		}

		entries, err := os.ReadDir(filepath.Join(profilesDir, profile.Name()))
		if err != nil {
			return fmt.Errorf("read service profile %s: %w", profile.Name(), err)
		}

		for _, entry := range entries {
			if strings.HasPrefix(strings.ToLower(entry.Name()), "ntuser") {
				err = os.RemoveAll(filepath.Join(profilesDir, profile.Name(), entry.Name()))
				if err != nil {
					return fmt.Errorf("remove profile hive: %w", err)
				}
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}

	return closeErr
}
