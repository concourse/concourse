package topgun_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A worker with a proxy configured", func() {
	BeforeEach(func() {
		Deploy("deployments/proxy-worker.yml")
	})

	It("uses the proxy server for executed tasks", func() {
		session := spawnFly("execute", "-c", "tasks/http-proxy.yml")
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))

		// don't actually expect the proxy to work, just that it tried it
		Expect(session).To(gbytes.Say("bad address 'proxy.example.com'"))
	})
})
