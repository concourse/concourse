package guidserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/garden/warden"
	. "github.com/onsi/gomega"
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

var container warden.Container

var ipAddress string

func Start(wardenClient warden.Client) {
	var err error

	container, err = wardenClient.Create(warden.ContainerSpec{
		RootFSPath: "docker:///concourse/testflight-helper",
	})
	Ω(err).ShouldNot(HaveOccurred())

	info, err := container.Info()
	Ω(err).ShouldNot(HaveOccurred())

	ipAddress = info.ContainerIP

	_, _, err = container.Run(warden.ProcessSpec{
		Path: "ruby",
		Args: []string{"-e", amazingRubyServer},
	})
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(func() error {
		_, err := http.Get(fmt.Sprintf("http://%s:8080/registrations", ipAddress))
		return err
	}, 2).ShouldNot(HaveOccurred())
}

func Stop(wardenClient warden.Client) {
	wardenClient.Destroy(container.Handle())

	container = nil
	ipAddress = ""
}

func CurlCommand() string {
	return fmt.Sprintf("curl -XPOST http://%s:8080/register -d @-", ipAddress)
}

func ReportingGuids() []string {
	uri := fmt.Sprintf("http://%s:8080/registrations", ipAddress)

	response, err := http.Get(uri)
	Ω(err).ShouldNot(HaveOccurred())

	defer response.Body.Close()

	var responses []string
	err = json.NewDecoder(response.Body).Decode(&responses)
	Ω(err).ShouldNot(HaveOccurred())

	return responses
}
