package k8s_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS Resolution", func() {

	var atc Endpoint

	BeforeEach(func() {
		setReleaseNameAndNamespace("dns")
	})

	AfterEach(func() {
		cleanupReleases()
		atc.Close()
	})

	var fullAddress = func() string {
		return releaseName + "-web." + namespace + ".svc.cluster.local:8080/api/v1/info"
	}

	const containerdRuntime = "containerd"
	const guardianRuntime = "guardian"

	DNSShouldWork := func(runtime string) {
		It("can reach local k8s services", func() {
			args := []string{
				`--set=worker.replicas=1`,
				`--set-string=concourse.worker.runtime=` + runtime,
			}
			deployConcourseChart(releaseName, args...)
			atc = waitAndLogin(namespace, releaseName+"-web")

			sess := fly.Start("execute", "-c", "tasks/dns-proxy-task.yml", "-v", "url="+fullAddress())
			<-sess.Exited

			Expect(sess.ExitCode()).To(BeZero())
		})
	}

	Context("with gdn backend", func() {
		DNSShouldWork(guardianRuntime)
	})

	Context("with containerd backend", func() {
		DNSShouldWork(containerdRuntime)
	})
})
