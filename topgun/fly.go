package topgun

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Fly struct {
	Bin    string
	Target string
	Home   string
}

type Worker struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func (f *Fly) Login(user, password, endpoint string) {
	Eventually(func() *gexec.Session {
		return f.Start(
			"login",
			"-c", endpoint,
			"-u", user,
			"-p", password,
		).Wait()
	}, 2*time.Minute, 10*time.Second).
		Should(gexec.Exit(0), "Fly should have been able to log in")
}

func (f *Fly) Run(argv ...string) {
	Wait(f.Start(argv...))
}

func (f *Fly) Start(argv ...string) *gexec.Session {
	return Start([]string{"HOME=" + f.Home}, f.Bin, append([]string{"--verbose", "-t", f.Target}, argv...)...)
}

func (f *Fly) SpawnInteractive(stdin io.Reader, argv ...string) *gexec.Session {
	return SpawnInteractive(stdin, []string{"HOME=" + f.Home}, f.Bin, append([]string{"--verbose", "-t", f.Target}, argv...)...)
}

func (f *Fly) GetWorkers() []Worker {
	var workers = []Worker{}

	sess := f.Start("workers", "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &workers)
	Expect(err).ToNot(HaveOccurred())

	return workers
}

func BuildBinary() string {
	flyBinPath, err := gexec.Build("github.com/concourse/concourse/fly")
	Expect(err).ToNot(HaveOccurred())

	return flyBinPath
}

func Start(env []string, command string, argv ...string) *gexec.Session {
	TimestampedBy("running: " + strings.Join(argv, " "))

	cmd := exec.Command(command, argv...)
	cmd.Env = env

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session
}

func SpawnInteractive(stdin io.Reader, env []string, command string, argv ...string) *gexec.Session {
	TimestampedBy("interactively running: " + command + " " + strings.Join(argv, " "))

	cmd := exec.Command(command, argv...)
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
