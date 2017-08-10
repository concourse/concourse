package acceptance_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Command", func() {
	Context("when telemtry is enabled", func() {
		It("submits the working version of concourse", func() {
			telemetryServer := ghttp.NewServer()
			defer telemetryServer.Close()

			telemetryServer.AppendHandlers(
				ghttp.VerifyRequest("GET",
					"/",
					"version=0.0.0-dev",
				),
			)

			os.Setenv("http_proxy", telemetryServer.URL())
			os.Setenv("https_proxy", telemetryServer.URL())

			atcCommand := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, true, BASIC_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
			defer atcCommand.Stop()

			Expect(telemetryServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when telemetry is disabled", func() {
		It("does not submit the working version of concourse", func() {
			telemetryServer := ghttp.NewServer()
			defer telemetryServer.Close()

			os.Setenv("http_proxy", telemetryServer.URL())
			os.Setenv("https_proxy", telemetryServer.URL())

			atcCommand := NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH)
			err := atcCommand.Start()
			Expect(err).NotTo(HaveOccurred())
			defer atcCommand.Stop()

			Expect(telemetryServer.ReceivedRequests()).To(HaveLen(0))
		})
	})
})
