package uidgid

import (
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Namespacer
type Namespacer interface {
	NamespacePath(logger lager.Logger, path string) error
	NamespacePathToUser(logger lager.Logger, path string, uid, gid int) error
	NamespaceCommand(cmd *exec.Cmd)
}

var _ Namespacer = (*UidNamespacer)(nil)

type UidNamespacer struct {
	Translator Translator
	Logger     lager.Logger
}

func (n *UidNamespacer) NamespacePath(logger lager.Logger, rootfsPath string) error {
	log := logger.Session("namespace", lager.Data{
		"path": rootfsPath,
	})

	log.Debug("start")
	defer log.Debug("done")

	if err := filepath.WalkDir(rootfsPath, n.Translator.TranslatePath); err != nil {
		log.Error("failed-to-walk-and-translate", err)
	}

	return nil
}

func (n *UidNamespacer) NamespacePathToUser(logger lager.Logger, rootfsPath string, uid, gid int) error {
	log := logger.Session("namespace", lager.Data{
		"path": rootfsPath,
		"uid":  uid,
		"gid":  gid,
	})

	log.Debug("start")
	defer log.Debug("done")

	if err := filepath.WalkDir(rootfsPath, n.Translator.TranslatePathToUser(uid, gid)); err != nil {
		log.Error("failed-to-walk-and-translate", err)
	}

	return nil
}

func (n *UidNamespacer) NamespaceCommand(cmd *exec.Cmd) {
	n.Translator.TranslateCommand(cmd)
}

var _ Namespacer = (*NoopNamespacer)(nil)

type NoopNamespacer struct{}

func (NoopNamespacer) NamespacePath(lager.Logger, string) error                 { return nil }
func (NoopNamespacer) NamespacePathToUser(lager.Logger, string, int, int) error { return nil }
func (NoopNamespacer) NamespaceCommand(cmd *exec.Cmd)                           {}
