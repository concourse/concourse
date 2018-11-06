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
		var atcs []boshInstance
		var atc0IP string
		var atc1IP string

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
				"-o", "operations/worker-rebalancing.yml",
			)

			waitForRunningWorker()

			atcs = JobInstances("web")
			atc0IP = atcs[0].IP
			atc1IP = atcs[1].IP

			atc0URL := "http://" + atcs[0].IP + ":8080"
			FlyLogin(atc0URL)
		})

		Describe("when a rebalance time is configured", func() {
			It("the worker eventually connects to both web nodes over a period of time", func() {
				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Minute, 5*time.Second).Should(Equal(atc0IP))

				Eventually(func() string {
					workers := flyTable("workers", "-d")
					return strings.Split(workers[0]["garden address"], ":")[0]
				}, time.Minute, 5*time.Second).Should(Equal(atc1IP))
			})
		})
	})
})
