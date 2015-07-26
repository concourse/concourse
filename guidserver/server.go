package guidserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/mgutz/ansi"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const gardenDeploymentIP = "10.244.15.2"

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
	addr      string
}

func Start(helperRootfs string, gardenClient garden.Client) *Server {
	container, err := gardenClient.Create(garden.ContainerSpec{
		RootFSPath: helperRootfs,
		GraceTime:  time.Hour,
	})
	Ω(err).ShouldNot(HaveOccurred())

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
	Ω(err).ShouldNot(HaveOccurred())

	hostPort, _, err := container.NetIn(0, 8080)
	Ω(err).ShouldNot(HaveOccurred())

	addr := fmt.Sprintf("%s:%d", gardenDeploymentIP, hostPort)

	Eventually(func() (int, error) {
		curl, err := container.Run(garden.ProcessSpec{
			Path: "curl",
			Args: []string{"-s", "-f", "http://127.0.0.1:8080/registrations"},
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
		Ω(err).ShouldNot(HaveOccurred())

		return curl.Wait()
	}).Should(Equal(0))

	return &Server{
		gardenClient: gardenClient,

		container: container,
		addr:      addr,
	}
}

func (server *Server) Stop() {
	server.gardenClient.Destroy(server.container.Handle())
}

func (server *Server) CurlCommand() string {
	return fmt.Sprintf("curl -XPOST http://%s/register -d @-", server.addr)
}

func (server *Server) ReportingGuids() []string {
	outBuf := new(bytes.Buffer)

	curl, err := server.container.Run(garden.ProcessSpec{
		Path: "curl",
		Args: []string{"-s", "-f", "http://127.0.0.1:8080/registrations"},
		User: "root",
	}, garden.ProcessIO{
		Stdout: outBuf,
		Stderr: gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[guid server polling]", "magenta")),
			ginkgo.GinkgoWriter,
		),
	})
	Ω(err).ShouldNot(HaveOccurred())

	Ω(curl.Wait()).Should(Equal(0))

	var responses []string
	err = json.NewDecoder(outBuf).Decode(&responses)
	Ω(err).ShouldNot(HaveOccurred())

	return responses
}
