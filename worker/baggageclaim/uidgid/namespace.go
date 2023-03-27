package uidgid

import (
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Namespacer

type Namespacer interface {
	NamespacePath(logger lager.Logger, path string) error
	NamespaceCommand(cmd *exec.Cmd)
}

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

	if err := filepath.Walk(rootfsPath, n.Translator.TranslatePath); err != nil {
		log.Error("failed-to-walk-and-translate", err)
	}

	return nil
}

func (n *UidNamespacer) NamespaceCommand(cmd *exec.Cmd) {
	n.Translator.TranslateCommand(cmd)
}

type NoopNamespacer struct{}

func (NoopNamespacer) NamespacePath(lager.Logger, string) error { return nil }
func (NoopNamespacer) NamespaceCommand(cmd *exec.Cmd)           {}
