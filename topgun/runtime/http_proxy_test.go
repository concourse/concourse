package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A worker with a proxy configured", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/worker-proxy.yml")
	})

	It("uses the proxy server for executed tasks", func() {
		session := Fly.Start("execute", "-c", "tasks/http-proxy.yml")
		// don't actually expect the proxy to work, just that it tried it
		Eventually(session).Should(gexec.Exit())
		Expect(session).To(gbytes.Say("bad address 'proxy.example.com'"))
	})
})
