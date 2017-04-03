package guidserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"
	"github.com/mgutz/ansi"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const amazingRubyServer = `
require 'webrick'
require 'json'

server = WEBrick::HTTPServer.new :Port => 8080

registered = []
files = {}

server.mount_proc '/register' do |req, res|
  registered << req.body.chomp
  res.status = 200
end

server.mount_proc '/registrations' do |req, res|
  res.body = JSON.generate(registered)
end

trap('INT') {
  server.shutdown
}

server.start
`

type Server struct {
	gardenClient garden.Client

	container garden.Container
	rootfsVol baggageclaim.Volume

	addr string
}

func Start(client concourse.Client) *Server {
	logger := lagertest.NewTestLogger("guid-server")

	rootfsPath, gardenClient, baggageclaimClient := helpers.WorkerWithResourceType(logger, client, "bosh-deployment")

	handle, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	rootfsVol, err := baggageclaimClient.CreateVolume(logger,
		handle.String(),
		baggageclaim.VolumeSpec{
			Strategy: baggageclaim.ImportStrategy{
				Path: rootfsPath,
			},
			Properties: baggageclaim.VolumeProperties{
				"testflight": "yep",
			},
		})
	Expect(err).NotTo(HaveOccurred())

	container, err := gardenClient.Create(garden.ContainerSpec{
		RootFSPath: (&url.URL{Scheme: "raw", Path: rootfsVol.Path()}).String(),
		GraceTime:  time.Hour,
		Properties: garden.Properties{
			"testflight": "yep",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = container.Run(garden.ProcessSpec{
		Path: "ruby",
		Args: []string{"-e", amazingRubyServer},
		User: "root",
	}, garden.ProcessIO{
		Stdout: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[guid server]", "magenta")),
			ginkgo.GinkgoWriter,
		),
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[guid server]", "magenta")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())

	info, err := container.Info()
	Expect(err).NotTo(HaveOccurred())

	addr := fmt.Sprintf("%s:%d", info.ContainerIP, 8080)

	Eventually(func() (int, error) {
		get, err := container.Run(garden.ProcessSpec{
			Path: "ruby",
			Args: []string{"-rnet/http", "-e", `Net::HTTP.get(URI("http://127.0.0.1:8080/registrations"))`},
			User: "root",
		}, garden.ProcessIO{
			Stdout: gexec.NewPrefixedWriter(
				fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[guid server polling]", "magenta")),
				ginkgo.GinkgoWriter,
			),
			Stderr: gexec.NewPrefixedWriter(
				fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[guid server polling]", "magenta")),
				ginkgo.GinkgoWriter,
			),
		})
		Expect(err).NotTo(HaveOccurred())

		return get.Wait()
	}).Should(Equal(0))

	return &Server{
		gardenClient: gardenClient,
		container:    container,
		rootfsVol:    rootfsVol,
		addr:         addr,
	}
}

func (server *Server) Stop() {
	err := server.gardenClient.Destroy(server.container.Handle())
	Expect(err).NotTo(HaveOccurred())

	server.rootfsVol.Release(baggageclaim.FinalTTL(time.Second))
}

func (server *Server) RegisterCommand() string {
	host, port, err := net.SplitHostPort(server.addr)
	Expect(err).ToNot(HaveOccurred())

	return fmt.Sprintf(`ruby -rnet/http -e 'Net::HTTP.start("%s", %s) { |http| puts http.post("/register", STDIN.read).body }'`, host, port)
}

func (server *Server) RegistrationsCommand() string {
	return fmt.Sprintf(`ruby -rnet/http -e 'puts Net::HTTP.get(URI("http://%s/registrations"))'`, server.addr)
}

func (server *Server) ReportingGuids() []string {
	outBuf := new(bytes.Buffer)

	get, err := server.container.Run(garden.ProcessSpec{
		Path: "ruby",
		Args: []string{"-rnet/http", "-e", `puts Net::HTTP.get(URI("http://127.0.0.1:8080/registrations"))`},
		User: "root",
	}, garden.ProcessIO{
		Stdout: outBuf,
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[guid server polling]", "magenta")),
			ginkgo.GinkgoWriter,
		),
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(get.Wait()).To(Equal(0))

	var responses []string
	err = json.NewDecoder(outBuf).Decode(&responses)
	Expect(err).NotTo(HaveOccurred())

	return responses
}

func Cleanup(client concourse.Client) {
	logger := lagertest.NewTestLogger("guid-server-cleanup")

	_, gardenClient, baggageclaimClient := helpers.WorkerWithResourceType(logger, client, "bosh-deployment")

	containers, err := gardenClient.Containers(garden.Properties{"testflight": "yep"})
	Expect(err).ToNot(HaveOccurred())

	for _, container := range containers {
		err := gardenClient.Destroy(container.Handle())
		Expect(err).ToNot(HaveOccurred())
	}

	volumes, err := baggageclaimClient.ListVolumes(logger, baggageclaim.VolumeProperties{"testflight": "yep"})
	Expect(err).ToNot(HaveOccurred())

	for _, volume := range volumes {
		err := volume.Destroy()
		Expect(err).ToNot(HaveOccurred())
	}
}
