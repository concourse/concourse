package gitserver

import (
	"bytes"
	"fmt"
	"net/url"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"
	"github.com/mgutz/ansi"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type Server struct {
	gardenClient garden.Client

	container garden.Container
	rootfsVol baggageclaim.Volume

	addr string

	committedGuids []string
}

func Start(client concourse.Client) *Server {
	logger := lagertest.NewTestLogger("git-server")

	gitServerRootfs, gardenClient, baggageclaimClient := helpers.WorkerWithResourceType(logger, client, "git")

	handle, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	rootfsVol, err := baggageclaimClient.CreateVolume(
		logger,
		handle.String(),
		baggageclaim.VolumeSpec{
			Strategy: baggageclaim.ImportStrategy{
				Path: gitServerRootfs,
			},
		})
	Expect(err).NotTo(HaveOccurred())

	container, err := gardenClient.Create(garden.ContainerSpec{
		RootFSPath: (&url.URL{Scheme: "raw", Path: rootfsVol.Path()}).String(),
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
		rootfsVol:    rootfsVol,
		addr:         fmt.Sprintf("%s:%d", info.ContainerIP, 9418),
	}
}

func (server *Server) Stop() {
	err := server.gardenClient.Destroy(server.container.Handle())
	Expect(err).NotTo(HaveOccurred())

	server.rootfsVol.Release(baggageclaim.FinalTTL(time.Second))
}

func (server *Server) URI() string {
	return fmt.Sprintf("git://%s/some-repo", server.addr)
}

func (server *Server) CommitOnBranch(branch string) string {
	guid, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{
			"-c",
			fmt.Sprintf(
				`
					cd some-repo
					git checkout -B '%s'
					echo '%s' >> guids
					git add guids
					git commit -m 'commit #%d: %s'
				`,
				branch,
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

func (server *Server) CommitFileToBranch(fileContents, fileName, branch string) {

	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{
			"-c",
			fmt.Sprintf(
				`
					cd some-repo
					git checkout -B '%s'
					cat > %s <<EOF
%s
EOF
					git add -A
					git commit -m "adding file %s"
				`,
				branch,
				fileName,
				fileContents,
				fileName,
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
}

func (server *Server) Commit() string {
	return server.CommitOnBranch("master")
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
				cp -a /usr rootfs/usr
				cp -a /root rootfs/root || true # prevent copy infinite loop
				rm -r rootfs/root/some-repo
				touch rootfs/hello-im-a-git-rootfs
				echo '{"env":["IMAGE_PROVIDED_ENV=hello-im-image-provided-env"]}' > metadata.json
				git checkout -B master
				git add rootfs metadata.json
				git commit -qm 'created rootfs'
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

func (server *Server) WriteFile(fileName string, fileContents string) {
	process, err := server.container.Run(garden.ProcessSpec{
		Path: "sh",
		Args: []string{
			"-exc",
			`cat > ` + fileName + ` <<EOF
` + fileContents + `
EOF
`,
		},
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[fly]", "blue")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[fly]", "blue")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))
}

func (server *Server) CommitResourceWithFile(fileName string) {

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
				cp -a /usr rootfs/usr
				cp -a /root rootfs/root || true # prevent copy infinite loop
				rm -r rootfs/root/some-repo
				echo '{}' > metadata.json

				mkdir -p rootfs/opt/resource
				echo some-bogus-version > rootfs/version
				echo fetched from custom resource > rootfs/some-file

				cat > rootfs/opt/resource/in <<EOF
#!/bin/sh

set -e -x

echo fetching using custom resource >&2

cd \$1
mkdir rootfs
cp -a /bin rootfs/bin
cp -a /etc rootfs/etc
cp -a /lib rootfs/lib
cp -a /lib64 rootfs/lib64
cp -a /usr rootfs/usr
cp -a /root rootfs/root
cp -a /some-file rootfs/some-file
echo '{"env":["SOME_ENV=yep"]}' > metadata.json

cat <<EOR
{"version":{"timestamp":"\$(date +%s)"},"metadata":[{"name":"some","value":"metadata"}]}
EOR
EOF

				cat > rootfs/opt/resource/out <<EOF
#!/bin/sh

set -e -x

echo pushing using custom resource >&2

cd \$1
find . >&2

cat <<EOR
{"version":{"timestamp":"\$(date +%s)"},"metadata":[{"name":"some","value":"metadata"}]}
EOR
EOF

				cat > rootfs/opt/resource/check <<EOF
#!/bin/sh

set -e -x

cat <<EOR
[{"version":"\$(cat /version)"}]
EOR
EOF

				chmod +x rootfs/opt/resource/*

				git checkout -B master
				git add rootfs metadata.json ` + fileName + `
				git commit -qm 'created resource rootfs'
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

// produce a rootfs implementing a resource versioned via unix timestamps, which does:
//
//   /in: produce a rootfs so that this resource can be used as an
//   image_resource
//
//   /out: print the stuff that we were told to push (doesn't actually do
//   anything)
//
//   /check: emit one and only one version so that the pipeline only triggers once
//
// this is used to test `get`, `put`, and `task` steps with custom resource types
func (server *Server) CommitResource() {
	server.CommitResourceWithFile("")
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
