package gitserver

import (
	"bytes"
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/mgutz/ansi"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type Server struct {
	gardenClient garden.Client
	container    garden.Container

	addr string

	committedGuids []string
}

func Start(helperRootfs string, gardenClient garden.Client) *Server {
	container, err := gardenClient.Create(garden.ContainerSpec{
		RootFSPath: helperRootfs,
		GraceTime:  time.Hour,
	})
	Expect(err).NotTo(HaveOccurred())

	process, err := container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{"-c", `
git config --global user.email dummy@example.com
git config --global user.name "Dummy User"

mkdir some-repo
cd some-repo
git init
touch .git/git-daemon-export-ok
`},
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git setup]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git setup]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))

	process, err = container.Run(garden.ProcessSpec{
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
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git server]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git server]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))

	info, err := container.Info()
	Expect(err).NotTo(HaveOccurred())

	return &Server{
		gardenClient: gardenClient,
		container:    container,
		addr:         fmt.Sprintf("%s:%d", info.ContainerIP, 9418),
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
	Expect(err).NotTo(HaveOccurred())

	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
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
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))

	server.committedGuids = append(server.committedGuids, guid.String())

	return guid.String()
}

func (server *Server) CommitRootfs() {
	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{
			"-exc",
			`
				cd some-repo
				mkdir rootfs
				cp -a /bin rootfs/bin
				cp -a /etc rootfs/etc
				cp -a /lib rootfs/lib
				cp -a /lib64 rootfs/lib64
				cp -a /root rootfs/root || true # prevent copy infinite loop
				touch rootfs/hello-im-a-git-rootfs
				git add rootfs
				git commit -m 'created rootfs'
			`,
		},
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[git commit]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))
}

func (server *Server) RevParse(ref string) string {
	buf := new(bytes.Buffer)

	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{"-e", "-c",
			fmt.Sprintf(
				`
					cd some-repo
					git rev-parse -q --verify %s
				`,
				ref,
			),
		},
		User: "root",
	}, garden.ProcessIO{
		Stdout: buf,
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "red+bright"), ansi.Color("[git rev-parse]", "green")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = process.Wait()
	Expect(err).NotTo(HaveOccurred())

	return buf.String()
}

func (server *Server) CommittedGuids() []string {
	return server.committedGuids
}
