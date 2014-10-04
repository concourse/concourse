package guidserver

import (
	"bytes"
	"encoding/json"
	"fmt"

	gapi "github.com/cloudfoundry-incubator/garden/api"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const gardenDeploymentIP = "10.244.16.2"

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

var container gapi.Container

var addr string

func Start(helperRootfs string, gardenClient gapi.Client) {
	var err error

	container, err = gardenClient.Create(gapi.ContainerSpec{
		RootFSPath: helperRootfs,
	})
	Ω(err).ShouldNot(HaveOccurred())

	_, err = container.Run(gapi.ProcessSpec{
		Path: "ruby",
		Args: []string{"-e", amazingRubyServer},
	}, gapi.ProcessIO{
		Stdout: ginkgo.GinkgoWriter,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())

	hostPort, _, err := container.NetIn(0, 8080)
	Ω(err).ShouldNot(HaveOccurred())

	addr = fmt.Sprintf("%s:%d", gardenDeploymentIP, hostPort)

	Eventually(func() (int, error) {
		curl, err := container.Run(gapi.ProcessSpec{
			Path: "curl",
			Args: []string{"http://127.0.0.1:8080/registrations"},
		}, gapi.ProcessIO{
			Stdout: ginkgo.GinkgoWriter,
			Stderr: ginkgo.GinkgoWriter,
		})
		Ω(err).ShouldNot(HaveOccurred())

		return curl.Wait()
	}, 2).Should(Equal(0))
}

func Stop(gardenClient gapi.Client) {
	gardenClient.Destroy(container.Handle())

	container = nil
	addr = ""
}

func CurlCommand() string {
	return fmt.Sprintf("curl -XPOST http://%s/register -d @-", addr)
}

func ReportingGuids() []string {
	outBuf := new(bytes.Buffer)

	curl, err := container.Run(gapi.ProcessSpec{
		Path: "curl",
		Args: []string{"http://127.0.0.1:8080/registrations"},
	}, gapi.ProcessIO{
		Stdout: outBuf,
		Stderr: ginkgo.GinkgoWriter,
	})
	Ω(err).ShouldNot(HaveOccurred())

	Ω(curl.Wait()).Should(Equal(0))

	var responses []string
	err = json.NewDecoder(outBuf).Decode(&responses)
	Ω(err).ShouldNot(HaveOccurred())

	return responses
}
