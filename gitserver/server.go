package gitserver

import (
	"bytes"
	"fmt"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const gardenDeploymentIP = "10.244.16.2"

type Server struct {
	gardenClient warden.Client
	container    warden.Container

	addr string

	committedGuids []string
}

func Start(helperRootfs string, gardenClient warden.Client) *Server {
	container, err := gardenClient.Create(warden.ContainerSpec{
		RootFSPath: helperRootfs,
	})
	Ω(err).ShouldNot(HaveOccurred())

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
		Args: []string{
			"daemon",
			"--export-all",
			"--enable=receive-pack",
			"--reuseaddr",
			"--base-path=.",
			"--detach",
			".",
		},
	}, warden.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	hostPort, _, err := container.NetIn(0, 9418)
	Ω(err).ShouldNot(HaveOccurred())

	return &Server{
		gardenClient: gardenClient,
		container:    container,
		addr:         fmt.Sprintf("%s:%d", gardenDeploymentIP, hostPort),
	}
}

func (server *Server) Stop() {
	server.gardenClient.Destroy(server.container.Handle())
}

func (server *Server) URI() string {
	return fmt.Sprintf("git://%s/some-repo", server.addr)
}

func (server *Server) Commit() string {
	guid, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())

	process, err := server.container.Run(warden.ProcessSpec{
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
				len(server.committedGuids)+1,
				guid,
			),
		},
	}, warden.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	server.committedGuids = append(server.committedGuids, guid.String())

	return guid.String()
}

func (server *Server) RevParse(ref string) string {
	buf := new(bytes.Buffer)

	process, err := server.container.Run(warden.ProcessSpec{
		Path: "git",
		Args: []string{"rev-parse", ref},
		Dir:  "some-repo",
	}, warden.ProcessIO{
		Stdout: buf,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())

	status, err := process.Wait()
	Ω(err).ShouldNot(HaveOccurred())

	if status == 0 {
		return buf.String()
	} else {
		// git rev-parse prints the input string if it cannot resolve it;
		// return an empty string instead
		return ""
	}
}

func (server *Server) CommittedGuids() []string {
	return server.committedGuids
}
