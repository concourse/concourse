package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
)

type runner struct {
	bin  string
	argv []string
}

func NewRunner(bin string, argv ...string) ifrit.Runner {
	return &runner{
		bin:  bin,
		argv: argv,
	}
}

func (r *runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	name := filepath.Base(r.bin)

	session, err := gexec.Start(
		exec.Command(r.bin, r.argv...),
		gexec.NewPrefixedWriter("\x1b[32m[o]\x1b[31m["+name+"]\x1b[0m ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("\x1b[91m[e]\x1b[31m["+name+"]\x1b[0m ", ginkgo.GinkgoWriter),
	)
	if err != nil {
		return err
	}

	close(ready)

dance:
	for {
		select {
		case sig := <-signals:
			session.Signal(sig)
		case <-session.Exited:
			break dance
		}
	}

	if session.ExitCode() == 0 {
		return nil
	}

	return fmt.Errorf("exit status %d", session.ExitCode())
}
