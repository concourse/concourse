package uidgid

import (
	"os"
	"os/exec"
)

//counterfeiter:generate . Translator

type Translator interface {
	// Translate current path's UID/GID to work inside the container's user
	// namespace
	TranslatePath(path string, dir os.DirEntry, err error) error
	// Translate current path's UID/GID to a specific UID/GID inside the
	// container's user namespace
	TranslatePathToUser(int, int) func(path string, dir os.DirEntry, err error) error
	TranslateCommand(*exec.Cmd)
}

var _ Translator = (*translator)(nil)

type translator struct {
	mapper Mapper
	chown  func(path string, uid int, gid int) error
	chmod  func(name string, mode os.FileMode) error
}

type Mapper interface {
	// Maps a UID:GID from a container's user namespace back to the host user namespace
	Map(int, int) (int, int)
	Apply(*exec.Cmd)
}

func NewTranslator(mapper Mapper) *translator {
	return &translator{
		mapper: mapper,
		chown:  os.Lchown,
		chmod:  os.Chmod,
	}
}

func (t *translator) TranslatePath(path string, dir os.DirEntry, err error) error {
	if err != nil {
		return err
	}

	info, err := dir.Info()
	if err != nil {
		return err
	}

	// We look at who the owner of the file/dir is in the host's user
	// namespace and then figure out what UID/GID that maps to in the
	// container's user namespace
	uid, gid := t.getuidgid(info)
	touid, togid := t.mapper.Map(uid, gid)

	if touid != uid || togid != gid {
		mode := info.Mode()
		t.chown(path, touid, togid)
		if mode&os.ModeSymlink == 0 {
			t.chmod(path, mode)
		}
	}

	return nil
}

func (t *translator) TranslatePathToUser(uid, gid int) func(path string, dir os.DirEntry, err error) error {
	touid, togid := t.mapper.Map(uid, gid)
	return func(path string, dir os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := dir.Info()
		if err != nil {
			return err
		}

		currentUid, currentGid := t.getuidgid(info)

		if touid != currentUid || togid != currentGid {
			mode := info.Mode()
			t.chown(path, touid, togid)
			if mode&os.ModeSymlink == 0 {
				t.chmod(path, mode)
			}
		}

		return nil
	}
}

func (t *translator) TranslateCommand(cmd *exec.Cmd) {
	t.setuidgid(cmd)
}
