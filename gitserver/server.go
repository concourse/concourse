package gitserver

import (
	"bytes"
	"fmt"

	gapi "github.com/cloudfoundry-incubator/garden/api"
	"github.com/mgutz/ansi"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const gardenDeploymentIP = "10.244.16.2"

type Server struct {
	gardenClient gapi.Client
	container    gapi.Container

	addr string

	committedGuids []string
}

func Start(helperRootfs string, gardenClient gapi.Client) *Server {
	container, err := gardenClient.Create(gapi.ContainerSpec{
		RootFSPath: helperRootfs,
	})
	Ω(err).ShouldNot(HaveOccurred())

	process, err := container.Run(gapi.ProcessSpec{
		Path: "bash",
		Args: []string{"-c", `
git config --global user.email dummy@example.com
git config --global user.name "Dummy User"

mkdir some-repo
cd some-repo
git init
touch .git/git-daemon-export-ok
`},
	}, gapi.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git setup]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git setup]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	process, err = container.Run(gapi.ProcessSpec{
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
	}, gapi.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git server]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git server]", "green")),
			ginkgo.GinkgoWriter,
		),
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

	process, err := server.container.Run(gapi.ProcessSpec{
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
	}, gapi.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Ω(err).ShouldNot(HaveOccurred())
	Ω(process.Wait()).Should(Equal(0))

	server.committedGuids = append(server.committedGuids, guid.String())

	return guid.String()
}

func (server *Server) RevParse(ref string) string {
	buf := new(bytes.Buffer)

	process, err := server.container.Run(gapi.ProcessSpec{
		Path: "git",
		Args: []string{"rev-parse", "-q", "--verify", ref},
		Dir:  "some-repo",
	}, gapi.ProcessIO{
		Stdout: buf,
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "red+bright"), ansi.Color("[git rev-parse]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Ω(err).ShouldNot(HaveOccurred())

	_, err = process.Wait()
	Ω(err).ShouldNot(HaveOccurred())

	return buf.String()
}

func (server *Server) CommittedGuids() []string {
	return server.committedGuids
}
