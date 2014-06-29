package guidserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/garden/warden"
	. "github.com/onsi/gomega"
)

const amazingRubyServer = `ruby <<END_MAGIC_SERVER
require 'webrick'
require 'json'

server = WEBrick::HTTPServer.new :Port => ENV['PORT']

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
END_MAGIC_SERVER
`

var container warden.Container

var hostPort uint32
var ipAddress string

func Start(wardenClient warden.Client) {
	var err error

	container, err = wardenClient.Create(warden.ContainerSpec{
		RootFSPath: "docker:///ubuntu#14.04",
	})
	Ω(err).ShouldNot(HaveOccurred())

	var containerPort uint32
	hostPort, containerPort, err = container.NetIn(0, 0)
	Ω(err).ShouldNot(HaveOccurred())

	info, err := container.Info()
	Ω(err).ShouldNot(HaveOccurred())

	ipAddress = info.ContainerIP

	_, _, err = container.Run(warden.ProcessSpec{
		Script: amazingRubyServer,
		EnvironmentVariables: []warden.EnvironmentVariable{
			warden.EnvironmentVariable{Key: "PORT", Value: fmt.Sprintf("%d", containerPort)},
		},
	})
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(func() error {
		_, err := http.Get(fmt.Sprintf("http://%s:%d/registrations", ipAddress, hostPort))
		return err
	}, 2).ShouldNot(HaveOccurred())
}

func Stop(wardenClient warden.Client) {
	wardenClient.Destroy(container.Handle())

	container = nil
	hostPort = 0
	ipAddress = ""
}

func CurlCommand() string {
	return fmt.Sprintf("curl -XPOST http://%s:%d/register -d @-", ipAddress, hostPort)
}

func ReportingGuids() []string {
	uri := fmt.Sprintf("http://%s:%d/registrations", ipAddress, hostPort)

	response, err := http.Get(uri)
	Ω(err).ShouldNot(HaveOccurred())

	defer response.Body.Close()

	var responses []string
	err = json.NewDecoder(response.Body).Decode(&responses)
	Ω(err).ShouldNot(HaveOccurred())

	return responses
}
