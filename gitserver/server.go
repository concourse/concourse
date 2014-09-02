package gitserver

import (
	"bytes"
	"fmt"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var container warden.Container

var ipAddress string

var committedGuids []string

func Start(helperRootfs string, wardenClient warden.Client) {
	var err error

	container, err = wardenClient.Create(warden.ContainerSpec{
		RootFSPath: helperRootfs,
	})
	Ω(err).ShouldNot(HaveOccurred())

	info, err := container.Info()
	Ω(err).ShouldNot(HaveOccurred())

	ipAddress = info.ContainerIP

	process, err := container.Run(warden.ProcessSpec{
		Path: "bash",
		Args: []string{"-c", `
git config --global user.email dummy@example.com
git config --global user.name "Dummy User"

mkdir some-repo
cd some-repo
git init
touch .git/git-daemon-export-ok
`},
	}, warden.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	process, err = container.Run(warden.ProcessSpec{
		Path: "git",
		Args: []string{"daemon", "--reuseaddr", "--base-path=.", "--detach", "."},
	}, warden.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))
}

func Stop(wardenClient warden.Client) {
	wardenClient.Destroy(container.Handle())

	container = nil
	ipAddress = ""
}

func URI() string {
	return fmt.Sprintf("git://%s/some-repo", ipAddress)
}

func Commit() {
	guid, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())

	process, err := container.Run(warden.ProcessSpec{
		Path: "bash",
		Args: []string{
			"-c",
			fmt.Sprintf(
				`
					cd some-repo
					echo '%s' >> guids
					git add guids
					git commit -m 'commit #%d: %s'
				`,
				guid,
				len(committedGuids)+1,
				guid,
			),
		},
	}, warden.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	committedGuids = append(committedGuids, guid.String())
}

func RevParse(ref string) string {
	buf := new(bytes.Buffer)

	process, err := container.Run(warden.ProcessSpec{
		Path: "git",
		Args: []string{"rev-parse", ref},
		Dir:  "some-repo",
	}, warden.ProcessIO{
		Stdout: buf,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())

	_, err = process.Wait()
	Ω(err).ShouldNot(HaveOccurred())

	return buf.String()
}

func CommittedGuids() []string {
	return committedGuids
}
