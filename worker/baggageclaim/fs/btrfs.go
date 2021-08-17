package fs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
)

type BtrfsFilesystem struct {
	imagePath string
	mountPath string
	mkfsBin   string

	logger lager.Logger
}

func New(logger lager.Logger, imagePath string, mountPath string, mkfsBin string) *BtrfsFilesystem {
	return &BtrfsFilesystem{
		imagePath: imagePath,
		mountPath: mountPath,
		mkfsBin:   mkfsBin,
		logger:    logger,
	}
}

// lower your expectations
func (fs *BtrfsFilesystem) Create(bytes uint64) error {

	// significantly
	idempotent := exec.Command("bash", "-e", "-x", "-c", `
		if [ ! -e $IMAGE_PATH ] || [ "$(stat --printf="%s" $IMAGE_PATH)" != "$SIZE_IN_BYTES" ]; then
			touch $IMAGE_PATH
			truncate -s ${SIZE_IN_BYTES} $IMAGE_PATH
		fi

		lo="$(losetup -j $IMAGE_PATH | cut -d':' -f1)"
		if [ -z "$lo" ]; then
			lo="$(losetup -f --show $IMAGE_PATH)"
		fi

		if ! file $IMAGE_PATH | grep BTRFS; then
			`+fs.mkfsBin+` --nodiscard $IMAGE_PATH
		fi

		mkdir -p $MOUNT_PATH

		if ! mountpoint -q $MOUNT_PATH; then
			mount -t btrfs -o discard $lo $MOUNT_PATH
		fi
	`)

	idempotent.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"MOUNT_PATH=" + fs.mountPath,
		"IMAGE_PATH=" + fs.imagePath,
		fmt.Sprintf("SIZE_IN_BYTES=%d", bytes),
	}

	_, err := fs.run(idempotent)
	return err
}

func (fs *BtrfsFilesystem) Delete() error {
	_, err := fs.run(exec.Command(
		"umount",
		fs.mountPath,
	))
	if err != nil {
		return err
	}

	if err := os.RemoveAll(fs.mountPath); err != nil {
		return err
	}

	loopbackOutput, err := fs.run(exec.Command(
		"losetup",
		"-j",
		fs.imagePath,
	))
	if err != nil {
		return err
	}

	loopbackDevice := strings.Split(loopbackOutput, ":")[0]

	_, err = fs.run(exec.Command(
		"losetup",
		"-d",
		loopbackDevice,
	))
	if err != nil {
		return err
	}

	return os.Remove(fs.imagePath)
}

func (fs *BtrfsFilesystem) run(cmd *exec.Cmd) (string, error) {
	logger := fs.logger.Session("run-command", lager.Data{
		"command": cmd.Path,
		"args":    cmd.Args,
		"env":     cmd.Env,
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	loggerData := lager.Data{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		logger.Error("failed", err, loggerData)
		return "", err
	}

	logger.Debug("ran", loggerData)

	return stdout.String(), nil
}
