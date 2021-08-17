package uidgid

import (
	"os"
	"os/exec"
)

//go:generate counterfeiter . Translator

type Translator interface {
	TranslatePath(path string, info os.FileInfo, err error) error
	TranslateCommand(*exec.Cmd)
}

type translator struct {
	mapper Mapper
	chown  func(path string, uid int, gid int) error
	chmod  func(name string, mode os.FileMode) error
}

type Mapper interface {
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

func (t *translator) TranslatePath(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

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

func (t *translator) TranslateCommand(cmd *exec.Cmd) {
	t.setuidgid(cmd)
}
