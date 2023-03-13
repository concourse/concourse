package topgun

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Start(env []string, command string, argv ...string) *gexec.Session {
	TimestampedBy("running: " + command + " " + strings.Join(argv, " "))

	cmd := exec.Command(command, argv...)
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	cmd.Dir = filepath.Join(cwd, "..")
	cmd.Env = env

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session
}

func SpawnInteractive(stdin io.Reader, env []string, command string, argv ...string) *gexec.Session {
	TimestampedBy("interactively running: " + command + " " + strings.Join(argv, " "))

	cmd := exec.Command(command, argv...)
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	cmd.Dir = filepath.Join(cwd, "..")
	cmd.Stdin = stdin
	cmd.Env = env

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func TimestampedBy(msg string) {
	By(fmt.Sprintf("[%.9f] %s", float64(time.Now().UnixNano())/1e9, msg))
}

func Wait(session *gexec.Session) {
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))
}

func Run(env []string, command string, argv ...string) *gexec.Session {
	session := Start(env, command, argv...)
	Wait(session)
	return session
}
