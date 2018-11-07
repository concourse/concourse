package topgun_test

import (
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rebalancing workers", func() {
	Context("with two TSAs available", func() {
		var webInstances []boshInstance

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
				"-o", "operations/worker-rebalancing.yml",
			)

			waitForRunningWorker()

			webInstances = JobInstances("web")
		})

		Describe("when a rebalance time is configured", func() {
			It("the worker eventually connects to both web nodes over a period of time", func() {
				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Minute, 5*time.Second).Should(SatisfyAny(
					Equal(webInstances[0].IP),
					Equal(webInstances[0].DNS),
				))

				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Minute, 5*time.Second).Should(SatisfyAny(
					Equal(webInstances[1].IP),
					Equal(webInstances[1].DNS),
				))
			})
		})
	})
})
